// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package test

import (
	_ "embed"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var tmpDir string

//go:embed configuration.yaml
var configFile string

var _log = logr.FromSlogHandler(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

var _ = BeforeSuite(func() {
	tmpDir = GinkgoT().TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "config.yaml"), []byte(configFile), 0444)
	Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(filepath.Join(tmpDir, "kubeconfig"), []byte("kubeconfig"), 0444)
	Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(filepath.Join(tmpDir, "token"), []byte("token"), 0444)
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, tmpDir)
})

func TestOidcApps(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OIDC Apps Webhook Suite")
}
