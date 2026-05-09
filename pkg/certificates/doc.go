// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

//go:generate go tool -modfile=../../tools/go.mod mockgen -package certificates -destination=mocks.go github.com/nickytd/oidc-apps/pkg/certificates CertificateOperations
package certificates
