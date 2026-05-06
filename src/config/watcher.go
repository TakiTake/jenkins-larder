package config

import (
	"context"
	"log/slog"
	"os"
	"time"
)

const (
	defaultPollInterval = 30 * time.Second
)

// Watcher monitors a config file for changes and emits updated configs on a channel.
type Watcher struct {
	filePath       string
	pollInterval   time.Duration
	updates        chan *Config
	done           chan struct{}
	lastModTime    time.Time
	initialConfig  *Config
}

// NewWatcher creates a new config watcher that polls the config file for changes.
// It emits the initial config immediately on the returned channel, then watches for updates.
func NewWatcher(filePath string, initialConfig *Config) (*Watcher, <-chan *Config, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, nil, err
	}

	w := &Watcher{
		filePath:      filePath,
		pollInterval:  defaultPollInterval,
		updates:       make(chan *Config, 1),
		done:          make(chan struct{}),
		lastModTime:   stat.ModTime(),
		initialConfig: initialConfig,
	}

	// Emit the initial config immediately
	w.updates <- initialConfig

	return w, w.updates, nil
}

// Start begins watching the config file for changes. Blocks until Stop is called.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case <-ticker.C:
			stat, err := os.Stat(w.filePath)
			if err != nil {
				slog.Error("failed to stat config file", "err", err)
				continue
			}

			if stat.ModTime().After(w.lastModTime) {
				w.lastModTime = stat.ModTime()
				cfg, err := Load(w.filePath)
				if err != nil {
					slog.Error("failed to reload config", "err", err)
					continue
				}

				select {
				case w.updates <- cfg:
				default:
					// Channel full, skip this update
					slog.Warn("config update channel full, skipping update")
				}
			}
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.done)
}
