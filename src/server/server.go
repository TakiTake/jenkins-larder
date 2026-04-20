package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourorg/jenkins-larder/src/admin"
	"github.com/yourorg/jenkins-larder/src/config"
)

// Server manages the HTTP endpoints for the mirror
type Server struct {
	config        *config.Config
	cache         *CacheService
	pluginServer  *http.Server
	adminServer   *http.Server
	metricsServer *http.Server
}

// New creates a new Server instance
func New(cfg *config.Config) (*Server, error) {
	// Initialize cache service
	cache, err := NewCacheService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache service: %w", err)
	}

	return &Server{
		config: cfg,
		cache:  cache,
	}, nil
}

// PluginHandler returns the HTTP handler for plugin download requests.
// Useful for testing with httptest.
func (s *Server) PluginHandler() http.Handler {
	mux := http.NewServeMux()
	downloadHandler := NewDownloadHandler(s.cache)
	mux.Handle("/download/plugins/", downloadHandler)
	return mux
}

// AdminHandler returns the HTTP handler for admin API requests.
// Useful for testing with httptest.
func (s *Server) AdminHandler() http.Handler {
	mux := http.NewServeMux()
	adminHandler := admin.NewAdminHandler(s.cache)
	mux.HandleFunc("/admin/cache/invalidate", adminHandler.InvalidateCacheHandler)
	mux.HandleFunc("/admin/cache/stats", adminHandler.CacheStatsHandler)
	mux.HandleFunc("/admin/health", adminHandler.HealthCheckHandler)
	return mux
}

// MetricsHandler returns the HTTP handler for Prometheus metrics.
// Useful for testing with httptest.
func (s *Server) MetricsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

// Start starts all HTTP servers (plugin, admin, metrics)
func (s *Server) Start() error {
	// Set up plugin download server
	pluginMux := http.NewServeMux()
	downloadHandler := NewDownloadHandler(s.cache)
	pluginMux.Handle("/download/plugins/", downloadHandler)

	s.pluginServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.Port),
		Handler:      pluginMux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute, // Allow time for large plugin downloads
	}

	// Set up admin server
	s.adminServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Admin.Port),
		Handler:      s.AdminHandler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Set up metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	s.metricsServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.MetricsPort),
		Handler:      metricsMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Start servers in goroutines
	errChan := make(chan error, 3)

	go func() {
		slog.Info("Starting plugin server", "port", s.config.Server.Port)
		if err := s.pluginServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("plugin server error: %w", err)
		}
	}()

	go func() {
		slog.Info("Starting admin server", "port", s.config.Admin.Port)
		if err := s.adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("admin server error: %w", err)
		}
	}()

	go func() {
		slog.Info("Starting metrics server", "port", s.config.Server.MetricsPort)
		if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("metrics server error: %w", err)
		}
	}()

	// Wait for any server error
	return <-errChan
}

// Shutdown gracefully shuts down all servers
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Shutting down servers...")

	errChan := make(chan error, 3)

	// Shutdown plugin server
	go func() {
		if s.pluginServer != nil {
			errChan <- s.pluginServer.Shutdown(ctx)
		} else {
			errChan <- nil
		}
	}()

	// Shutdown admin server
	go func() {
		if s.adminServer != nil {
			errChan <- s.adminServer.Shutdown(ctx)
		} else {
			errChan <- nil
		}
	}()

	// Shutdown metrics server
	go func() {
		if s.metricsServer != nil {
			errChan <- s.metricsServer.Shutdown(ctx)
		} else {
			errChan <- nil
		}
	}()

	// Wait for all shutdowns to complete
	var lastErr error
	for i := 0; i < 3; i++ {
		if err := <-errChan; err != nil {
			slog.Error("Server shutdown error", "error", err)
			lastErr = err
		}
	}

	return lastErr
}
