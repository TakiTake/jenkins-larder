package config

import (
	"errors"
	"net/url"
)

// TODO (T009): Implement configuration validation

// ValidateStorageConfig validates storage configuration
func ValidateStorageConfig(cfg *StorageConfig) error {
	if cfg.LimitBytes <= 0 {
		return errors.New("storage.limit_bytes must be greater than 0")
	}

	if cfg.Path == "" {
		return errors.New("storage.path cannot be empty")
	}

	return nil
}

// ValidateServerPorts validates server port configuration
func ValidateServerPorts(server *ServerConfig, admin *AdminConfig) error {
	// Validate server port
	if server.Port < 1 || server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}

	// Validate metrics port
	if server.MetricsPort < 1 || server.MetricsPort > 65535 {
		return errors.New("server.metrics_port must be between 1 and 65535")
	}

	// Validate admin port
	if admin.Port < 1 || admin.Port > 65535 {
		return errors.New("admin.port must be between 1 and 65535")
	}

	// Check for port conflicts
	if server.Port == server.MetricsPort {
		return errors.New("server.port and server.metrics_port cannot be the same")
	}
	if server.Port == admin.Port {
		return errors.New("server.port and admin.port cannot be the same")
	}
	if server.MetricsPort == admin.Port {
		return errors.New("server.metrics_port and admin.port cannot be the same")
	}

	return nil
}

// ValidateUpstreamConfig validates upstream configuration
func ValidateUpstreamConfig(cfg *UpstreamConfig) error {
	if cfg.URL == "" {
		return errors.New("upstream.url cannot be empty")
	}

	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return errors.New("invalid upstream URL")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("upstream URL must use http or https scheme")
	}

	if cfg.TimeoutSeconds < 10 || cfg.TimeoutSeconds > 300 {
		return errors.New("upstream.timeout_seconds must be between 10 and 300")
	}

	return nil
}

// ValidateLarderConfig validates Larder unified configuration
func ValidateLarderConfig(cfg *LarderConfig) error {
	if cfg.RSA.KeyPath == "" {
		return errors.New("larder.rsa.key_path cannot be empty")
	}

	if cfg.RSA.CertPath == "" {
		return errors.New("larder.rsa.cert_path cannot be empty")
	}

	if cfg.TLS.Enabled {
		if cfg.TLS.CertPath == "" {
			return errors.New("larder.tls.cert_path cannot be empty when tls is enabled")
		}
		if cfg.TLS.KeyPath == "" {
			return errors.New("larder.tls.key_path cannot be empty when tls is enabled")
		}
	}

	if cfg.UpdateCenter.BaseURL != "" {
		baseURL, err := url.Parse(cfg.UpdateCenter.BaseURL)
		if err != nil {
			return errors.New("invalid larder update_center base URL")
		}
		if baseURL.Scheme != "http" && baseURL.Scheme != "https" {
			return errors.New("larder update_center base URL must use http or https scheme")
		}
	}

	if cfg.UpdateCenter.TTLSeconds < 60 {
		return errors.New("larder.update_center.ttl_seconds must be at least 60")
	}

	return nil
}
