// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"sync"

	"github.com/nickytd/oidc-apps/pkg/randutils"
)

var (
	generatedCAName, generatedTLSName string
	onceCA, onceTLS                   sync.Once
)

func generateCACommonName(prefix string) string {
	onceCA.Do(func() {
		generatedCAName = prefix + "-" + generateRandomString(5)
	})

	return generatedCAName
}

func generateTLSCommonName(prefix string) string {
	onceTLS.Do(func() {
		generatedTLSName = prefix + "-" + generateRandomString(5)
	})

	return generatedTLSName
}

func generateRandomString(length int) string {
	return randutils.GenerateRandomString(length)
}
