package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourorg/jenkins-larder/src/config"
	"github.com/yourorg/jenkins-larder/src/server"
)

func main() {
	// Jenkins Larder - A well-stocked larder for your Jenkins plugins

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/default.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if cfg.Larder.UpdateCenter.BaseURL == "" {
		ip, err := config.ContainerIP()
		if err != nil {
			log.Fatalf("Failed to detect container IP: %v", err)
		}
		cfg.Larder.UpdateCenter.BaseURL = fmt.Sprintf("http://%s:%d", ip, cfg.Server.Port)
	}

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting Jenkins Larder",
		"storage_limit_gb", cfg.Storage.LimitBytes/(1024*1024*1024),
		"upstream_url", cfg.Upstream.URL,
		"update_center_base_url", cfg.Larder.UpdateCenter.BaseURL,
	)

	// Setup config watcher for hot reload
	watcher, _, err := config.NewWatcher(configPath, cfg)
	if err != nil {
		log.Fatalf("Failed to create config watcher: %v", err)
	}
	go watcher.Start(context.Background())
	defer watcher.Stop()

	// Create unified Larder server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	slog.Info("Server started successfully",
		"plugin_port", cfg.Server.Port,
		"metrics_port", cfg.Server.MetricsPort,
	)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down gracefully...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped")
}
