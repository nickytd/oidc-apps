// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/go-logr/logr"
	"k8s.io/component-base/version"

	"github.com/nickytd/oidc-apps/pkg/watcher/hashcalc"
	"github.com/nickytd/oidc-apps/pkg/watcher/parameters"
	"github.com/nickytd/oidc-apps/pkg/watcher/process"
)

var (
	proc     *process.Process
	procLock sync.Mutex
	log      logr.Logger
)

func setupLogger(params parameters.Parameters) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(params.LogLevel)); err != nil {
		level = slog.LevelInfo
	}

	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if params.LogFormat == "text" {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	}

	log = logr.FromSlogHandler(handler).WithName("watcher")
}

func main() {
	params := parameters.Parse(os.Args)
	setupLogger(params)

	log.Info("kube-rbac-proxy-watcher started",
		"version", version.Get().GitVersion,
		"revision", version.Get().GitCommit,
		"gitTreeState", version.Get().GitTreeState,
	)

	log.Info(
		"child process parameters",
		"watchedDir", params.WatchedDir,
		"cmdLine", params.CmdLine,
		"cmdLineArgs", params.CmdLineArgs,
	)

	proc = process.New(log, params.CmdLine, params.CmdLineArgs...)
	sigs := make(chan os.Signal, 1)

	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan bool, 1)

	ctx, cancel := setupContext()
	defer cancel()

	go handleSignals(sigs, done)

	hash := hashcalc.RunTotalHashCalc(ctx, params.WatchedDir)
	currentHash := <-hash

	if err := startProcess(); err != nil {
		cancel()

		return
	}

	monitorHashChanges(hash, currentHash, done, params)
}

func setupContext() (context.Context, context.CancelFunc) {
	c, cancel := context.WithCancel(context.Background()) // #nosec G118 -- cancel is returned and deferred by caller
	ctx := logr.NewContext(c, log)

	return ctx, cancel
}

func handleSignals(sigs chan os.Signal, done chan bool) {
	sig := <-sigs
	log.Info(
		"signal received",
		"signal", sig.String(),
	)

	procLock.Lock()

	if proc != nil {
		_ = proc.Stop()
	}

	procLock.Unlock()

	done <- true
}

func startProcess() error {
	procLock.Lock()
	defer procLock.Unlock()

	if err := proc.Start(); err != nil {
		log.Error(err, "error starting the child process")

		return err
	}

	return nil
}

func monitorHashChanges(hash <-chan string, currentHash string, done chan bool, params parameters.Parameters) {
	for {
		select {
		case <-done:
			log.Info("exiting")

			return
		case h := <-hash:
			if currentHash != h {
				log.Info(
					"total hash changed",
					"old hash", currentHash,
					"new hash", h,
				)

				currentHash = h

				restartProcess(params)
			}
		}
	}
}

func restartProcess(params parameters.Parameters) {
	procLock.Lock()
	defer procLock.Unlock()

	if err := proc.Stop(); err != nil {
		log.Error(err, "error stopping child process")

		return
	}

	proc = process.New(log, params.CmdLine, params.CmdLineArgs...)

	if err := proc.Start(); err != nil {
		log.Error(err, "error starting child process")

		return
	}
}
