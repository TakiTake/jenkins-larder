package unit

import (
	"testing"

	"github.com/yourorg/jenkins-larder/src/config"
)

func TestValidateServerPorts(t *testing.T) {
	tests := []struct {
		name    string
		server  config.ServerConfig
		admin   config.AdminConfig
		wantErr bool
	}{
		{
			name:    "valid ports",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: false,
		},
		{
			name:    "server port too low",
			server:  config.ServerConfig{Port: 0, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: true,
		},
		{
			name:    "server port too high",
			server:  config.ServerConfig{Port: 70000, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: true,
		},
		{
			name:    "metrics port too low",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 0},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: true,
		},
		{
			name:    "metrics port too high",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 70000},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: true,
		},
		{
			name:    "admin port too low",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 0},
			wantErr: true,
		},
		{
			name:    "admin port too high",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 70000},
			wantErr: true,
		},
		{
			name:    "server equals metrics",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 8080},
			admin:   config.AdminConfig{Port: 8081},
			wantErr: true,
		},
		{
			name:    "server equals admin",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 8080},
			wantErr: true,
		},
		{
			name:    "metrics equals admin",
			server:  config.ServerConfig{Port: 8080, MetricsPort: 9090},
			admin:   config.AdminConfig{Port: 9090},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateServerPorts(&tt.server, &tt.admin)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServerPorts() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateUpstreamConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.UpstreamConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 60},
			wantErr: false,
		},
		{
			name:    "timeout at lower bound",
			cfg:     config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 10},
			wantErr: false,
		},
		{
			name:    "timeout at upper bound",
			cfg:     config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 300},
			wantErr: false,
		},
		{
			name:    "timeout too high",
			cfg:     config.UpstreamConfig{URL: "https://updates.jenkins.io", TimeoutSeconds: 301},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.ValidateUpstreamConfig(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpstreamConfig() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
