// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"encoding/base64"

	"k8s.io/utils/ptr"
)

// GetOidcCASecretName returns the secret name holding the trusts CA certificate of the OIDC Provider
func (rc *ResolvedConfig) GetOidcCASecretName() string {
	secretName := ""

	if rc.target.OidcCASecretRef != nil &&
		rc.target.OidcCASecretRef.Name != "" {
		return rc.target.OidcCASecretRef.Name
	}

	if rc.global.OidcCASecretRef != nil &&
		rc.global.OidcCASecretRef.Name != "" {
		secretName = rc.global.OidcCASecretRef.Name
	}

	return secretName
}

// GetOidcCABundle returns the trusted CA bundle certificates of the OIDC Provider
func (rc *ResolvedConfig) GetOidcCABundle() string {
	var (
		decodedBytes []byte
		err          error
		oidcCABundle string
	)

	if rc.target.OidcCABundle != "" {
		if decodedBytes, err = base64.StdEncoding.DecodeString(rc.target.OidcCABundle); err != nil {
			rc.log.Error(err, "failed to decode oidc ca bundle")

			return ""
		}

		return string(decodedBytes)
	}

	if rc.global.OidcCABundle != "" {
		oidcCABundle = rc.global.OidcCABundle
	}

	if decodedBytes, err = base64.StdEncoding.DecodeString(oidcCABundle); err != nil {
		rc.log.Error(err, "failed to decode oidc ca bundle")

		return ""
	}

	return string(decodedBytes)
}

// GetClientID returns the OIDC Provider client_id for the given workload target
func (rc *ResolvedConfig) GetClientID() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.ClientID != "" {
		return rc.target.Oauth2Proxy.ClientID
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.ClientID != "" {
		return rc.global.Oauth2Proxy.ClientID
	}

	return ""
}

// GetClientSecret returns the OIDC Provider secret for the given target workload
func (rc *ResolvedConfig) GetClientSecret() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.ClientSecret != "" {
		return rc.target.Oauth2Proxy.ClientSecret
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.ClientSecret != "" {
		return rc.global.Oauth2Proxy.ClientSecret
	}

	return ""
}

// GetScope returns the OIDC Provider scope for the given target workload
func (rc *ResolvedConfig) GetScope() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.Scope != "" {
		return rc.target.Oauth2Proxy.Scope
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.Scope != "" {
		return rc.global.Oauth2Proxy.Scope
	}

	return ""
}

// GetRedirectURL returns the OIDC Provider redirect URL for the given workload target
func (rc *ResolvedConfig) GetRedirectURL() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.RedirectURL != "" {
		return rc.target.Oauth2Proxy.RedirectURL
	}

	// The redirect URL shall not default to the global one.
	// Instead, it shall be constructed as below code */
	// If the target oidc configuration does not define a redirect URL
	// it will be constructed as https://{name}-{namespace}.domainName/oauth2/callback
	return "https://" + rc.GetHost() + "/oauth2/callback"
}

// GetOidcIssuerURL returns the OIDC Provider URL for the given workload target
func (rc *ResolvedConfig) GetOidcIssuerURL() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.OidcIssuerURL != "" {
		return rc.target.Oauth2Proxy.OidcIssuerURL
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.OidcIssuerURL != "" {
		return rc.global.Oauth2Proxy.OidcIssuerURL
	}

	return ""
}

// GetSslInsecureSkipVerify designates if oauth2-proxy shall skip upstream ssl validation
func (rc *ResolvedConfig) GetSslInsecureSkipVerify() bool {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.SSLInsecureSkipVerify != nil {
		return ptr.Deref(rc.target.Oauth2Proxy.SSLInsecureSkipVerify, false)
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.SSLInsecureSkipVerify != nil {
		return ptr.Deref(rc.global.Oauth2Proxy.SSLInsecureSkipVerify, false)
	}

	return false
}

// GetInsecureOidcSkipIssuerVerification designates if oauth2-proxy shall skip OIDC Provider certificate validation
func (rc *ResolvedConfig) GetInsecureOidcSkipIssuerVerification() bool {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.InsecureOidcSkipIssuerVerification != nil {
		return ptr.Deref(rc.target.Oauth2Proxy.InsecureOidcSkipIssuerVerification, false)
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.InsecureOidcSkipIssuerVerification != nil {
		return ptr.Deref(rc.global.Oauth2Proxy.InsecureOidcSkipIssuerVerification, false)
	}

	return false
}

// GetInsecureOidcSkipNonce designates if oauth2-proxy shall skip OIDC nonce request parameter
func (rc *ResolvedConfig) GetInsecureOidcSkipNonce() bool {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.InsecureOidcSkipNonce != nil {
		return ptr.Deref(rc.target.Oauth2Proxy.InsecureOidcSkipNonce, false)
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.InsecureOidcSkipNonce != nil {
		return ptr.Deref(rc.global.Oauth2Proxy.InsecureOidcSkipNonce, false)
	}

	return false
}

// GetApprovalPrompt returns the OIDC approval prompt mode for the given workload target
func (rc *ResolvedConfig) GetApprovalPrompt() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.ApprovalPrompt != "" {
		return rc.target.Oauth2Proxy.ApprovalPrompt
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.ApprovalPrompt != "" {
		return rc.global.Oauth2Proxy.ApprovalPrompt
	}

	return "auto"
}

// GetCookieRefresh returns the oauth2-proxy cookie refresh interval for the given workload target
func (rc *ResolvedConfig) GetCookieRefresh() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.CookieRefresh != "" {
		return rc.target.Oauth2Proxy.CookieRefresh
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.CookieRefresh != "" {
		return rc.global.Oauth2Proxy.CookieRefresh
	}

	return "3600s"
}

// GetEmailDomain returns the email domain restriction for the given workload target
func (rc *ResolvedConfig) GetEmailDomain() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.EmailDomain != "" {
		return rc.target.Oauth2Proxy.EmailDomain
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.EmailDomain != "" {
		return rc.global.Oauth2Proxy.EmailDomain
	}

	return "*"
}

// GetSkipProviderButton returns whether to skip the oauth2-proxy provider selection button
func (rc *ResolvedConfig) GetSkipProviderButton() bool {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.SkipProviderButton != nil {
		return ptr.Deref(rc.target.Oauth2Proxy.SkipProviderButton, true)
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.SkipProviderButton != nil {
		return ptr.Deref(rc.global.Oauth2Proxy.SkipProviderButton, true)
	}

	return true
}

// GetCodeChallengeMethod returns the PKCE code challenge method for the given workload target
func (rc *ResolvedConfig) GetCodeChallengeMethod() string {
	if rc.target.Oauth2Proxy != nil &&
		rc.target.Oauth2Proxy.CodeChallengeMethod != "" {
		return rc.target.Oauth2Proxy.CodeChallengeMethod
	}

	if rc.global.Oauth2Proxy != nil &&
		rc.global.Oauth2Proxy.CodeChallengeMethod != "" {
		return rc.global.Oauth2Proxy.CodeChallengeMethod
	}

	return "S256"
}

// GetExtraArgs returns extra CLI arguments for oauth2-proxy for the given workload target
func (rc *ResolvedConfig) GetExtraArgs() []string {
	if rc.target.Oauth2Proxy != nil &&
		len(rc.target.Oauth2Proxy.ExtraArgs) > 0 {
		return rc.target.Oauth2Proxy.ExtraArgs
	}

	if rc.global.Oauth2Proxy != nil &&
		len(rc.global.Oauth2Proxy.ExtraArgs) > 0 {
		return rc.global.Oauth2Proxy.ExtraArgs
	}

	return nil
}
