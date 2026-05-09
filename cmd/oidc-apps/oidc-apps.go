// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	oidcappscontroller "github.com/nickytd/oidc-apps/pkg/oidc-apps"
)

var (
	_log    = logf.Log
	rootCmd *cobra.Command
)

func init() {
	rootCmd = newRootCommand()
}

func newRootCommand() *cobra.Command {
	opts := &oidcappscontroller.Options{}

	var (
		logLevelStr string
		logFormat   string
	)

	cmd := &cobra.Command{
		Use:           "oidc-apps",
		Short:         "This controller enhances target workloads with authentication & authorization proxies.",
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var level slog.Level
			if err := level.UnmarshalText([]byte(logLevelStr)); err != nil {
				level = slog.LevelInfo
			}

			handlerOpts := &slog.HandlerOptions{Level: level}

			var handler slog.Handler
			if logFormat == "text" {
				handler = slog.NewTextHandler(os.Stderr, handlerOpts)
			} else {
				handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
			}

			logf.SetLogger(logr.FromSlogHandler(handler))

			_log.Info("started",
				"version", version.Get().GitVersion,
				"revision", version.Get().GitCommit,
				"gitTreeState", version.Get().GitTreeState,
			)

			verflag.PrintAndExitIfRequested()

			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				_log.Info(fmt.Sprintf("FLAG: --%s=%s", flag.Name, flag.Value))
			})

			return oidcappscontroller.RunController(cmd.Context(), opts)
		},
	}

	verflag.AddFlags(cmd.Flags())
	opts.AddFlags(cmd.Flags())
	cmd.Flags().StringVar(&logLevelStr, "log-level", "info", "log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&logFormat, "log-format", "json", "log output format (json, text)")

	return cmd
}
