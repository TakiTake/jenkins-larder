# Quickstart Guide: Jenkins Plugin Cache Mirror

**Last Updated**: 2025-12-13
**Target Audience**: DevOps engineers deploying the mirror server

---

## Overview

This guide walks through deploying the Jenkins Plugin Cache Mirror in a Kubernetes cluster and configuring Jenkins instances to use it.

### Prerequisites

- Kubernetes cluster with PersistentVolume support
- `kubectl` configured to access the cluster
- Jenkins instances running in the same cluster (or with network access to the mirror service)
- Optional: Prometheus Operator for automatic metrics scraping

---

## Step 1: Deploy the Mirror Server

### Create Namespace

```bash
kubectl create namespace jenkins-infrastructure
```

### Deploy PersistentVolumeClaim

Create `pvc.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: jenkins-mirror-storage
  namespace: jenkins-infrastructure
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: standard  # Adjust to your storage class
```

Apply:

```bash
kubectl apply -f pvc.yaml
```

### Deploy ConfigMap

Create `configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jenkins-mirror-config
  namespace: jenkins-infrastructure
data:
  config.yaml: |
    storage:
      limit_bytes: 10737418240  # 10GB
      path: /var/cache/jenkins-plugins

    upstream:
      url: https://updates.jenkins.io
      timeout_seconds: 60

    server:
      port: 8080
      metrics_port: 9090

    admin:
      port: 8081

    ttl:
      enabled: false
      default_hours: 0  # No automatic TTL
```

Apply:

```bash
kubectl apply -f configmap.yaml
```

### Deploy Mirror Server

Create `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jenkins-mirror
  namespace: jenkins-infrastructure
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jenkins-mirror
  template:
    metadata:
      labels:
        app: jenkins-mirror
    spec:
      containers:
      - name: mirror
        image: your-registry/jenkins-mirror:latest  # Replace with actual image
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        - containerPort: 8081
          name: admin
        volumeMounts:
        - name: storage
          mountPath: /var/cache/jenkins-plugins
        - name: config
          mountPath: /etc/jenkins-mirror
        env:
        - name: CONFIG_PATH
          value: /etc/jenkins-mirror/config.yaml
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /admin/health
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /admin/health
            port: 8081
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: storage
        persistentVolumeClaim:
          claimName: jenkins-mirror-storage
      - name: config
        configMap:
          name: jenkins-mirror-config
```

Apply:

```bash
kubectl apply -f deployment.yaml
```

### Expose as Service

Create `service.yaml`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: jenkins-mirror
  namespace: jenkins-infrastructure
  labels:
    app: jenkins-mirror
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: metrics
  - port: 8081
    targetPort: 8081
    protocol: TCP
    name: admin
  selector:
    app: jenkins-mirror
```

Apply:

```bash
kubectl apply -f service.yaml
```

### Verify Deployment

```bash
# Check pod status
kubectl get pods -n jenkins-infrastructure

# Check logs
kubectl logs -n jenkins-infrastructure -l app=jenkins-mirror

# Test health endpoint
kubectl exec -n jenkins-infrastructure deployment/jenkins-mirror -- \
  curl -s http://localhost:8081/admin/health
```

Expected output:

```json
{"status":"healthy","uptime_seconds":120}
```

---

## Step 2: Configure Jenkins to Use the Mirror

### Option A: Environment Variable (Recommended for Kubernetes)

Update your Jenkins deployment to set `JENKINS_UC_DOWNLOAD`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jenkins
spec:
  template:
    spec:
      containers:
      - name: jenkins
        image: jenkins/jenkins:lts
        env:
        - name: JENKINS_UC_DOWNLOAD
          value: "http://jenkins-mirror.jenkins-infrastructure.svc.cluster.local:8080"
```

### Option B: Jenkins Configuration File

Edit `/var/jenkins_home/hudson.model.UpdateCenter.xml`:

```xml
<?xml version='1.1' encoding='UTF-8'?>
<sites>
  <site>
    <id>default</id>
    <url>http://jenkins-mirror.jenkins-infrastructure.svc.cluster.local:8080/update-center.json</url>
  </site>
</sites>
```

### Option C: Init Container (For plugin pre-installation)

```yaml
initContainers:
- name: install-plugins
  image: jenkins/jenkins:lts
  command:
    - sh
    - -c
    - |
      export JENKINS_UC_DOWNLOAD=http://jenkins-mirror.jenkins-infrastructure.svc.cluster.local:8080
      jenkins-plugin-cli --plugins git:5.0.0 workflow-aggregator:2.7
  volumeMounts:
  - name: jenkins-home
    mountPath: /var/jenkins_home
```

### Verify Jenkins Uses Mirror

1. Check Jenkins logs during plugin installation:

```bash
kubectl logs -n default deployment/jenkins | grep "Downloading plugin"
```

Expected: URLs should point to `jenkins-mirror.jenkins-infrastructure.svc.cluster.local`

2. Check mirror logs:

```bash
kubectl logs -n jenkins-infrastructure -l app=jenkins-mirror
```

Expected:

```
{"level":"info","msg":"cache miss","plugin":"git","version":"5.0.0","time":"2025-12-13T10:30:00Z"}
{"level":"info","msg":"downloading from upstream","url":"https://updates.jenkins.io/download/plugins/git/5.0.0/git.hpi"}
{"level":"info","msg":"cache hit","plugin":"git","version":"5.0.0","response_time_ms":52}
```

---

## Step 3: Monitor with Prometheus

### Create ServiceMonitor (if using Prometheus Operator)

Create `servicemonitor.yaml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: jenkins-mirror
  namespace: jenkins-infrastructure
spec:
  selector:
    matchLabels:
      app: jenkins-mirror
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

Apply:

```bash
kubectl apply -f servicemonitor.yaml
```

### View Metrics

```bash
# Port-forward to access metrics
kubectl port-forward -n jenkins-infrastructure svc/jenkins-mirror 9090:9090

# In another terminal
curl http://localhost:9090/metrics
```

Example metrics:

```
mirror_requests_total{status="hit"} 850
mirror_requests_total{status="miss"} 150
mirror_storage_bytes_used 5368709120
mirror_cached_plugins_total 512
```

### Create Grafana Dashboard

Import dashboard JSON or create panels for:

- **Cache Hit Rate**: `rate(mirror_requests_total{status="hit"}[5m]) / rate(mirror_requests_total[5m])`
- **Storage Usage**: `mirror_storage_bytes_used / mirror_storage_bytes_limit`
- **P95 Latency**: `histogram_quantile(0.95, mirror_request_duration_seconds_bucket{status="hit"})`
- **Bandwidth Saved**: `rate(mirror_bandwidth_saved_bytes_total[1h])`

---

## Step 4: Test Cache Behavior

### Test 1: Cache Miss → Cache Hit

```bash
# Install a plugin for the first time (cache miss)
kubectl exec -n default deployment/jenkins -- \
  java -jar /usr/share/jenkins/jenkins-cli.jar -s http://localhost:8080/ \
  install-plugin git:5.0.0

# Check mirror logs - should show "cache miss" then "downloading from upstream"

# Install same plugin on another Jenkins instance (cache hit)
kubectl exec -n default deployment/jenkins-2 -- \
  java -jar /usr/share/jenkins/jenkins-cli.jar -s http://localhost:8080/ \
  install-plugin git:5.0.0

# Check mirror logs - should show "cache hit" with fast response time
```

### Test 2: Manual Cache Invalidation

```bash
# Port-forward admin API
kubectl port-forward -n jenkins-infrastructure svc/jenkins-mirror 8081:8081

# Invalidate a cached plugin
curl -X POST http://localhost:8081/admin/cache/invalidate \
  -H "Content-Type: application/json" \
  -d '{"name":"git","version":"5.0.0"}'

# Response:
# {"status":"invalidated","plugin":"git:5.0.0","bytes_freed":10485760}

# Next request will re-download from upstream
```

### Test 3: Check Cache Statistics

```bash
curl http://localhost:8081/admin/cache/stats
```

Example response:

```json
{
  "total_requests": 1005,
  "cache_hits": 850,
  "cache_misses": 150,
  "errors": 5,
  "hit_rate": 0.846,
  "storage_used_bytes": 5368709120,
  "storage_limit_bytes": 10737418240,
  "cached_plugins": 512,
  "evictions": 23,
  "upstream_failures": 2,
  "checksum_failures": 0
}
```

---

## Step 5: Verify Success Criteria

### SC-001: Initialization Time Reduction

Measure Jenkins startup time before/after mirror:

```bash
# Without mirror (baseline)
time kubectl exec deployment/jenkins -- jenkins-plugin-cli --plugins git:5.0.0

# With mirror (should be ~50% faster on subsequent runs)
time kubectl exec deployment/jenkins-2 -- jenkins-plugin-cli --plugins git:5.0.0
```

### SC-002: Hit Rate > 80%

```bash
# Check after 1 week
kubectl exec -n jenkins-infrastructure deployment/jenkins-mirror -- \
  curl -s http://localhost:8081/admin/cache/stats | jq '.hit_rate'
```

Expected: `> 0.80`

### SC-006: P95 Latency < 500ms

Query Prometheus:

```promql
histogram_quantile(0.95, rate(mirror_request_duration_seconds_bucket{status="hit"}[5m]))
```

Expected: `< 0.5` (500ms)

---

## Troubleshooting

### Issue: Jenkins still downloads from internet

**Check**:

```bash
# Verify JENKINS_UC_DOWNLOAD is set
kubectl exec deployment/jenkins -- env | grep JENKINS_UC_DOWNLOAD
```

**Expected**: `http://jenkins-mirror.jenkins-infrastructure.svc.cluster.local:8080`

**Fix**: Update deployment environment variables.

### Issue: Mirror returns 502 Bad Gateway

**Check**:

```bash
# Test upstream connectivity from mirror pod
kubectl exec -n jenkins-infrastructure deployment/jenkins-mirror -- \
  curl -I https://updates.jenkins.io
```

**If fails**: Check firewall/proxy settings, ensure mirror pod has internet access.

### Issue: Storage full (507 errors)

**Check**:

```bash
curl http://localhost:8081/admin/cache/stats | jq '.storage_used_bytes, .storage_limit_bytes'
```

**Fix**:

```bash
# Increase storage limit in ConfigMap
kubectl edit configmap jenkins-mirror-config -n jenkins-infrastructure

# Update storage.limit_bytes to 20GB (21474836480)
# Restart deployment
kubectl rollout restart deployment/jenkins-mirror -n jenkins-infrastructure
```

### Issue: High upstream failure rate

**Check mirror logs**:

```bash
kubectl logs -n jenkins-infrastructure -l app=jenkins-mirror | grep upstream_failure
```

**Common causes**:
- Upstream timeout too low (increase `upstream.timeout_seconds` in config)
- Network issues between mirror and updates.jenkins.io
- Upstream update center outage (check https://status.jenkins.io)

---

## Next Steps

- Configure alerts for cache hit rate < 60% or upstream failures > 10/hour
- Set up log aggregation (ELK/Loki) to analyze plugin request patterns
- Consider horizontal scaling if serving > 100 Jenkins instances (requires ReadWriteMany PVC)
- Review and adjust storage limit based on actual usage after 1 month

---

## Additional Resources

- **API Documentation**: [contracts/mirror-api.yaml](./contracts/mirror-api.yaml)
- **Data Model**: [data-model.md](./data-model.md)
- **Research & Architecture**: [research.md](./research.md)
- **Jenkins Plugin Installation Docs**: https://github.com/jenkinsci/docker#preinstalling-plugins
