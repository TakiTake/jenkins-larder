package unit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
)

func TestConfigLoad(t *testing.T) {
	t.Run("loads valid YAML config", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "config.yaml")
		content := `
storage:
  limit_bytes: 1073741824
  path: /tmp/test-cache
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
  default_hours: 0
`
		if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.Storage.LimitBytes != 1073741824 {
			t.Errorf("expected LimitBytes=1073741824, got %d", cfg.Storage.LimitBytes)
		}
		if cfg.Storage.Path != "/tmp/test-cache" {
			t.Errorf("expected Path=/tmp/test-cache, got %s", cfg.Storage.Path)
		}
		if cfg.Upstream.URL != "https://updates.jenkins.io" {
			t.Errorf("expected upstream URL=https://updates.jenkins.io, got %s", cfg.Upstream.URL)
		}
		if cfg.Server.Port != 8080 {
			t.Errorf("expected port=8080, got %d", cfg.Server.Port)
		}
		if cfg.Admin.Port != 8081 {
			t.Errorf("expected admin port=8081, got %d", cfg.Admin.Port)
		}
	})

	t.Run("fails on missing file", func(t *testing.T) {
		_, err := config.Load("/nonexistent/path.yaml")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("fails on invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "bad.yaml")
		if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := config.Load(cfgPath)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("fails on invalid config values", func(t *testing.T) {
		dir := t.TempDir()
		cfgPath := filepath.Join(dir, "bad.yaml")
		content := `
storage:
  limit_bytes: 0
  path: ""
upstream:
  url: ""
  timeout_seconds: 0
server:
  port: 0
  metrics_port: 0
admin:
  port: 0
`
		if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := config.Load(cfgPath)
		if err == nil {
			t.Fatal("expected validation error")
		}
	})
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: false,
		},
		{
			name: "zero storage limit",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 0, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "empty storage path",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: ""},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "empty upstream URL",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "invalid upstream scheme",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "ftp://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "timeout too low",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 5},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "duplicate ports",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 8080, MetricsPort: 8080},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
		{
			name: "port out of range",
			cfg: config.Config{
				Storage:  config.StorageConfig{LimitBytes: 1024, Path: "/tmp/cache"},
				Upstream: config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
				Server:   config.ServerConfig{Port: 70000, MetricsPort: 9090},
				Admin:    config.AdminConfig{Port: 8081},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
