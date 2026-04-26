package contract

import (
	"context"
	"testing"
	"time"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

func TestServerStartAndShutdown(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100 * 1024 * 1024, Path: cacheDir},
		Upstream: config.UpstreamConfig{URL: "http://127.0.0.1:1", TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 0, MetricsPort: 0},
		Admin:    config.AdminConfig{Port: 0},
	}

	// Use port 0 won't work with Start() since it uses fmt.Sprintf(":%d", port)
	// and port 0 means "any available port" which is fine for ListenAndServe
	cfg.Server.Port = 19876
	cfg.Server.MetricsPort = 19877
	cfg.Admin.Port = 19878

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error: %v", err)
	}
}

func TestServerNewWithInvalidStorage(t *testing.T) {
	cfg := &config.Config{
		Storage:  config.StorageConfig{LimitBytes: 100, Path: "/dev/null/impossible"},
		Upstream: config.UpstreamConfig{URL: "http://localhost", TimeoutSeconds: 10},
		Server:   config.ServerConfig{Port: 8080, MetricsPort: 9090},
		Admin:    config.AdminConfig{Port: 8081},
	}

	_, err := server.New(cfg)
	if err == nil {
		t.Fatal("expected error with invalid storage path")
	}
}
