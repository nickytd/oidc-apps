// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	_ "go.uber.org/automaxprocs"
	runtimelog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func main() {
	ctx := signals.SetupSignalHandler()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		runtimelog.Log.Error(err, "error executing the main command")
		os.Exit(1)
	}
}
