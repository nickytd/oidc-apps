// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

var config *OIDCAppsControllerConfig
var once sync.Once

// Options is an option setter function
type Options func(config *OIDCAppsControllerConfig)

// WithClient supports setting client.Client option
func WithClient(c client.Client) Options {
	return func(config *OIDCAppsControllerConfig) {
		config.client = c
	}
}

// WithLog supports setting default logger
func WithLog(l logr.Logger) Options {
	return func(config *OIDCAppsControllerConfig) {
		config.log = l
	}
}

// CreateControllerConfigOrDie initializes the targets configurations or exits the controller when unsuccessful
func CreateControllerConfigOrDie(path string, opts ...Options) *OIDCAppsControllerConfig {
	once.Do(func() {
		config = &OIDCAppsControllerConfig{}
		for _, o := range opts {
			o(config)
		}

		cf, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			handleError(err, "failed to read extension configuration", path)
		}

		if err = yaml.Unmarshal(cf, config); err != nil {
			handleError(err, "failed to unmarshal extension configuration", path)
		}
	})

	return config
}

// SetClient sets the client for the configuration
func (c *OIDCAppsControllerConfig) SetClient(cl client.Client) {
	c.client = cl
}

// SetLogger sets the logger for the configuration
func (c *OIDCAppsControllerConfig) SetLogger(l logr.Logger) {
	c.log = l
}

func handleError(err error, message, path string) {
	if config.log.IsZero() {
		handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
		log.SetLogger(logr.FromSlogHandler(handler))

		config.log = log.Log.WithName("oidcAppsExtensionConfig")
	}

	config.log.Error(err, message, "path", path)
	panic("terminating")
}

// GetOIDCAppsControllerConfig returns the loaded configuration
func GetOIDCAppsControllerConfig() *OIDCAppsControllerConfig {
	return config
}
