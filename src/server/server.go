package server

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourorg/jenkins-larder/src/admin"
	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/updatecenter"
)

// Server manages the HTTP endpoints for the unified Larder service
type Server struct {
	config         *config.Config
	cache          *CacheService
	key            *rsa.PrivateKey
	cert           *x509.Certificate
	certDER        []byte
	signer         *updatecenter.Signer
	ucCache        *ucCache
	upstreamClient *http.Client
	pluginServer   *http.Server
	adminServer    *http.Server
	metricsServer  *http.Server
}

// ucCache stores the rendered and signed update-center.json with TTL.
type ucCache struct {
	mu     sync.Mutex
	data   []byte
	expiry time.Time
}

// New creates a new unified Server instance
func New(cfg *config.Config) (*Server, error) {
	// Initialize cache service
	cache, err := NewCacheService(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache service: %w", err)
	}

	// Load RSA key and certificate for update-center.json signing
	key, cert, certDER, err := loadKeyAndCert(cfg.Larder.RSA.KeyPath, cfg.Larder.RSA.CertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load RSA key and certificate: %w", err)
	}

	// Create signer
	signer := updatecenter.NewSigner(key, cert)

	// Create upstream HTTP client
	upstreamClient := &http.Client{
		Timeout: time.Duration(cfg.Upstream.TimeoutSeconds) * time.Second,
	}

	return &Server{
		config:         cfg,
		cache:          cache,
		key:            key,
		cert:           cert,
		certDER:        certDER,
		signer:         signer,
		ucCache:        newUCCache(),
		upstreamClient: upstreamClient,
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

	// Add update-center.json handler
	pluginMux.HandleFunc("/update-center.json", s.handleUpdateCenter)
	pluginMux.HandleFunc("/update-center-ca.crt", s.handleCert)

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

// Handler methods for update-center.json

func (s *Server) handleUpdateCenter(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check cache
	if cached := s.ucCache.get(); cached != nil {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(cached); err != nil {
			slog.Error("failed to write cached update center", "err", err)
		}
		return
	}

	// Fetch from upstream
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	raw, err := updatecenter.Fetch(ctx, s.upstreamClient, s.config.Upstream.URL+"/update-center.json")
	if err != nil {
		slog.Error("failed to fetch update center", "err", err)
		http.Error(w, "failed to fetch update center", http.StatusInternalServerError)
		return
	}

	// Parse
	uc, err := updatecenter.Parse(raw)
	if err != nil {
		slog.Error("failed to parse update center", "err", err)
		http.Error(w, "failed to parse update center", http.StatusInternalServerError)
		return
	}

	// Rewrite URLs
	if err := updatecenter.RewritePluginURLs(uc, s.config.Larder.UpdateCenter.BaseURL); err != nil {
		slog.Error("failed to rewrite URLs", "err", err)
		http.Error(w, "failed to rewrite URLs", http.StatusInternalServerError)
		return
	}

	// Sign
	if err := s.signer.Sign(uc); err != nil {
		slog.Error("failed to sign update center", "err", err)
		http.Error(w, "failed to sign update center", http.StatusInternalServerError)
		return
	}

	// Render
	rendered, err := updatecenter.Render(uc)
	if err != nil {
		slog.Error("failed to render update center", "err", err)
		http.Error(w, "failed to render update center", http.StatusInternalServerError)
		return
	}

	// Cache
	ttl := time.Duration(s.config.Larder.UpdateCenter.TTLSeconds) * time.Second
	s.ucCache.set(rendered, ttl)

	// Serve
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(rendered); err != nil {
		slog.Error("failed to write update center", "err", err)
	}
}

func (s *Server) handleCert(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/x-x509-ca-cert")
	w.Header().Set("Content-Disposition", "attachment; filename=larder.crt")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(s.certDER); err != nil {
		slog.Error("failed to write certificate", "err", err)
	}
}

// Helper functions for update-center.json handling

func newUCCache() *ucCache {
	return &ucCache{}
}

func (c *ucCache) set(data []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
	c.expiry = time.Now().Add(ttl)
}

func (c *ucCache) get() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().After(c.expiry) {
		return nil
	}

	return c.data
}

func loadKeyAndCert(keyPath, certPath string) (*rsa.PrivateKey, *x509.Certificate, []byte, error) {
	// Load key using the keygen module
	keyPEM, err := loadKeyPEM(keyPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load key: %w", err)
	}

	// Load cert using the keygen module
	cert, certDER, err := loadCertPEM(certPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	return keyPEM, cert, certDER, nil
}

func loadKeyPEM(keyPath string) (*rsa.PrivateKey, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("invalid key PEM format")
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}

func loadCertPEM(certPath string) (*x509.Certificate, []byte, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, nil, fmt.Errorf("invalid certificate PEM format")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, certBlock.Bytes, nil
}
