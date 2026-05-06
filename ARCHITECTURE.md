# Larder Architecture

## Overall System Design

```mermaid
graph TB
    subgraph jenkins_ns["Jenkins Namespace (jenkins-controller)"]
        JenkinsInit["initContainer<br/>(fetch cert)"]
        Jenkins["Jenkins Controller<br/>Port: 8080"]
        JenkinsHome["JENKINS_HOME<br/>PVC"]
        
        JenkinsInit -->|curl /update-center-ca.crt| Agent
        JenkinsInit -->|write to| UpdateCenterCAs["update-center-rootCAs/<br/>larder-agent.crt"]
        UpdateCenterCAs -->|mount| JenkinsHome
        
        Jenkins -->|GET /update-center.json| Agent
        Jenkins -->|GET /download/plugins/...| Agent
    end
    
    subgraph larder_ns["Larder Namespace"]
        subgraph Agent_Pod["Larder Agent Pod"]
            Agent["Agent Server<br/>Port: 8080"]
            AgentSigner["RSA Signer<br/>(tls.key, tls.crt)"]
            UCCache["UC Cache<br/>(in-memory TTL)"]
        end
        
        subgraph Controller_Pod["Larder Controller Pod"]
            Controller["Controller Server<br/>Port: 8080"]
            LRUCache["LRU Cache<br/>(filesystem)"]
            Storage["Plugin Storage<br/>limit_bytes"]
        end
        
        Secret["Secret: larder-signing-key<br/>(tls.key, tls.crt)"]
        ConfigMap["ConfigMap: larder-config<br/>(upstream, ports, TTLs)"]
        
        Secret -->|mount| Agent
        ConfigMap -->|mount| Agent
        ConfigMap -->|mount| Controller
        
        Agent -->|fetch update-center.json| UpstreamJSON["https://updates.jenkins.io<br/>/update-center.json"]
        Agent -->|parse, rewrite URLs| AgentSigner
        AgentSigner -->|sign with RSA| UCCache
        Agent -->|proxy requests| Controller
        
        Controller -->|cache miss| UpstreamHPI["https://updates.jenkins.io<br/>/download/plugins/..."]
        Controller -->|store in| LRUCache
    end
    
    subgraph Internet["Internet"]
        UpstreamJSON
        UpstreamHPI
    end
    
    style jenkins_ns fill:#e1f5ff
    style larder_ns fill:#f3e5f5
    style Agent_Pod fill:#fff9c4
    style Controller_Pod fill:#c8e6c9
    style Internet fill:#ffebee
```

## Request Flow Sequence

```mermaid
sequenceDiagram
    participant J as Jenkins Pod
    participant IC as InitContainer
    participant A as Larder Agent
    participant C as Larder Controller
    participant U as updates.jenkins.io
    
    Note over J,U: Pod Startup
    IC->>A: GET /update-center-ca.crt
    A-->>IC: DER Certificate
    IC->>IC: Write to $JENKINS_HOME/update-center-rootCAs/
    
    Note over J,U: First Update Center Request
    J->>A: GET /update-center.json
    A->>U: GET /update-center.json
    U-->>A: JSONP (updateCenter.post(...))
    A->>A: Parse JSONP → JSON
    A->>A: Rewrite URLs to Agent base_url
    A->>A: Sign with RSA (SHA-512 + SHA-1)
    A->>A: Cache in memory (TTL: 1 hour)
    A-->>J: Signed JSONP
    J->>J: Verify signature (cert in update-center-rootCAs/)
    
    Note over J,U: Plugin Download Flow
    J->>A: GET /download/plugins/git/1.0/git.hpi
    A->>C: GET /download/plugins/git/1.0/git.hpi
    
    alt Cache Hit
        C-->>A: .hpi from filesystem
    else Cache Miss
        C->>U: GET /download/plugins/git/1.0/git.hpi
        U-->>C: .hpi binary
        C->>C: Verify checksum
        C->>C: Store in LRU cache
        C-->>A: .hpi
    end
    
    A-->>J: .hpi (streamed)
    
    Note over J,U: Subsequent UC Requests (within TTL)
    J->>A: GET /update-center.json
    A-->>J: Cached JSONP (no upstream call)
```

## Component Interactions

```mermaid
graph LR
    subgraph "Jenkins (User Namespace)"
        JC["Jenkins Controller"]
    end
    
    subgraph "Larder Agent (larder ns)"
        UC["UpdateCenter<br/>Handler"]
        PX["Plugin Proxy<br/>Handler"]
        SG["Signer<br/>(RSA)"]
        CH["UC Cache<br/>(TTL)"]
    end
    
    subgraph "Larder Controller (larder ns)"
        DH["Download<br/>Handler"]
        CS["CacheService"]
        STG["Storage<br/>(LRU)"]
        UP["Upstream<br/>Client"]
    end
    
    subgraph "External"
        UJSON["updates.jenkins.io<br/>JSON"]
        UHPI["updates.jenkins.io<br/>Plugins"]
    end
    
    JC -->|1. GET /update-center.json| UC
    UC -->|2a. Fetch| UJSON
    UC -->|2b. Parse, Rewrite| SG
    SG -->|2c. Sign| CH
    CH -->|3. Return JSONP| JC
    
    JC -->|4. GET /download/plugins/...| PX
    PX -->|5. Proxy| DH
    DH -->|6. Get or Download| CS
    CS -->|6a. Cache hit| STG
    CS -->|6b. Cache miss| UP
    UP -->|6c. Fetch| UHPI
    UP -->|6d. Store| STG
    STG -->|7. Return bytes| PX
    PX -->|8. Stream to Jenkins| JC
    
    style JC fill:#e1f5ff
    style UC fill:#fff9c4
    style PX fill:#fff9c4
    style SG fill:#fff9c4
    style CH fill:#fff9c4
    style DH fill:#c8e6c9
    style CS fill:#c8e6c9
    style STG fill:#c8e6c9
    style UP fill:#c8e6c9
```

## Data Flow: Update Center JSON Processing

```mermaid
graph TB
    Start["Upstream<br/>update-center.json<br/><br/>updateCenter.post({<br/>  plugins: {<br/>    git: {<br/>      url: https://updates.jenkins.io/<br/>        download/plugins/git/1.0/git.hpi<br/>    }<br/>  }<br/>})"] 
    
    Parse["Parse: Remove JSONP wrapper<br/>Extract JSON object"]
    
    Rewrite["Rewrite URLs<br/><br/>https://updates.jenkins.io/ →<br/>http://larder-agent.svc/"]
    
    Sign["Sign with RSA Key<br/><br/>Remove old signature<br/>Compute SHA-1 & SHA-512 digests<br/>Sign both with RSA<br/>Embed certificate<br/>Add signature block"]
    
    Render["Render back to JSONP<br/><br/>updateCenter.post({<br/>  plugins: {...},<br/>  signature: {<br/>    certificates: [base64],<br/>    correct_digest: base64,<br/>    correct_digest512: hex,<br/>    correct_signature: base64,<br/>    correct_signature512: hex<br/>  }<br/>})"]
    
    Cache["Cache in Memory<br/><br/>TTL: 3600 seconds<br/>(configurable)"]
    
    Serve["Serve to Jenkins<br/><br/>Content-Type:<br/>text/javascript"]
    
    Start --> Parse
    Parse --> Rewrite
    Rewrite --> Sign
    Sign --> Render
    Render --> Cache
    Cache --> Serve
    
    style Start fill:#ffebee
    style Parse fill:#fff3e0
    style Rewrite fill:#fffde7
    style Sign fill:#fff9c4
    style Render fill:#f1f8e9
    style Cache fill:#e0f2f1
    style Serve fill:#e1f5fe
```

## Kubernetes Resources

```mermaid
graph TB
    subgraph Secrets["Secrets (larder namespace)"]
        SK["larder-signing-key<br/>tls.key: base64(private key)<br/>tls.crt: base64(certificate)"]
    end
    
    subgraph ConfigMaps["ConfigMaps (larder namespace)"]
        CM["larder-config<br/>upstream.url<br/>storage.limit_bytes<br/>server.port<br/>agent.base_url<br/>agent.controller_url<br/>agent.ttl_seconds"]
    end
    
    subgraph Deployments["Deployments (larder namespace)"]
        DP1["larder-agent<br/>replicas: 1<br/>- Container: larder<br/>  mode: agent<br/>  volumeMounts:<br/>    - tls (Secret)<br/>    - config (ConfigMap)"]
        
        DP2["larder-controller<br/>replicas: 1<br/>- Container: larder<br/>  mode: controller<br/>  volumeMounts:<br/>    - config (ConfigMap)<br/>    - cache (PVC)"]
    end
    
    subgraph Jenkins["Deployments (jenkins namespace)"]
        JDEP["jenkins-controller<br/>initContainers:<br/>  - fetch-larder-cert<br/>containers:<br/>  - jenkins<br/>  volumeMounts:<br/>    - jenkins-home (PVC)"]
    end
    
    SK -->|mount as volume| DP1
    CM -->|mount as volume| DP1
    CM -->|mount as volume| DP2
    
    JDEP -->|initContainer fetches cert from| DP1
    JDEP -->|configures Update Center URL| DP1
    
    style SK fill:#ffccbc
    style CM fill:#c5cae9
    style DP1 fill:#fff9c4
    style DP2 fill:#c8e6c9
    style JDEP fill:#e1f5ff
```

## Hot Reload Configuration Flow

```mermaid
sequenceDiagram
    participant Ops as Operator
    participant K8S as Kubernetes API
    participant CM as ConfigMap Volume
    participant Agent as Agent Pod
    participant Controller as Controller Pod
    
    Ops->>K8S: kubectl edit configmap larder-config
    Ops->>K8S: Change update_center_ttl: 3600
    K8S->>CM: Update mounted files
    
    par Agent Polling
        Agent->>Agent: Poll ConfigMap mtime (every 30s)
        Agent->>Agent: Detect change
        Agent->>Agent: Reload config
        Agent->>Agent: Update TTL setting
    and Controller Polling
        Controller->>Controller: Poll ConfigMap mtime (every 30s)
        Controller->>Controller: Detect change
        Controller->>Controller: Reload config
    end
    
    Note over Agent,Controller: No pod restart needed!
```

## Multi-Jenkins Deployment

```mermaid
graph TB
    subgraph larder_ns["Larder Namespace"]
        Agent["1x Larder Agent<br/>(shared by all Jenkins)"]
        Controller["1x Larder Controller<br/>(central cache)"]
    end
    
    subgraph jenkins_ns1["Jenkins Namespace 1"]
        J1["Jenkins-1"]
        I1["initContainer"]
    end
    
    subgraph jenkins_ns2["Jenkins Namespace 2"]
        J2["Jenkins-2"]
        I2["initContainer"]
    end
    
    subgraph jenkins_ns3["Jenkins Namespace N"]
        J3["Jenkins-N"]
        I3["initContainer"]
    end
    
    I1 -->|fetch cert| Agent
    J1 -->|GET /update-center.json<br/>GET /download/plugins/...| Agent
    
    I2 -->|fetch cert| Agent
    J2 -->|GET /update-center.json<br/>GET /download/plugins/...| Agent
    
    I3 -->|fetch cert| Agent
    J3 -->|GET /update-center.json<br/>GET /download/plugins/...| Agent
    
    Agent -->|proxy| Controller
    
    style larder_ns fill:#f3e5f5
    style jenkins_ns1 fill:#e1f5ff
    style jenkins_ns2 fill:#e1f5ff
    style jenkins_ns3 fill:#e1f5ff
```

## Certificate Trust Chain

```mermaid
graph LR
    subgraph Generation["Key Generation & Trust"]
        KG["larder-keygen<br/>generates RSA key pair"]
        SK["Kubernetes Secret<br/>larder-signing-key"]
        
        KG -->|creates| SK
    end
    
    subgraph Agent["Agent Pod"]
        Key["Private Key<br/>(tls.key)"]
        Cert["Certificate<br/>(tls.crt)"]
        
        SK -->|mount| Key
        SK -->|mount| Cert
    end
    
    subgraph Jenkins["Jenkins Pod"]
        RootCAs["update-center-rootCAs/<br/>larder-agent.crt"]
        Verify["Signature Verification"]
        
        Cert -->|fetch via initContainer| RootCAs
        RootCAs -->|trust chain| Verify
    end
    
    UC["Update Center JSON<br/>with RSA signature"]
    
    Cert -->|sign| UC
    UC -->|serve to Jenkins| Verify
    
    style Generation fill:#ffccbc
    style Agent fill:#fff9c4
    style Jenkins fill:#e1f5ff
```
