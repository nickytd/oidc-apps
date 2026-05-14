// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package configuration

import (
	"encoding/base64"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetOidcCASecretName returns the secret name holding the trusts CA certificate of the OIDC Provider
func (c *OIDCAppsControllerConfig) GetOidcCASecretName(object client.Object) string {
	secretName := ""

	t := c.FetchTarget(object)
	if t.OidcCASecretRef != nil &&
		t.OidcCASecretRef.Name != "" {
		return t.OidcCASecretRef.Name
	}

	if c.Global.OidcCASecretRef != nil &&
		c.Global.OidcCASecretRef.Name != "" {
		secretName = c.Global.OidcCASecretRef.Name
	}

	return secretName
}

// GetOidcCABundle returns the trusted CA bundle certificates of the OIDC Provider
func (c *OIDCAppsControllerConfig) GetOidcCABundle(object client.Object) string {
	var (
		decodedBytes []byte
		err          error
		oidcCABundle string
	)

	t := c.FetchTarget(object)
	if t.OidcCABundle != "" {
		if decodedBytes, err = base64.StdEncoding.DecodeString(t.OidcCABundle); err != nil {
			c.log.Error(err, "failed to decode oidc ca bundle")

			return ""
		}

		return string(decodedBytes)
	}

	if c.Global.OidcCABundle != "" {
		oidcCABundle = c.Global.OidcCABundle
	}

	if decodedBytes, err = base64.StdEncoding.DecodeString(oidcCABundle); err != nil {
		c.log.Error(err, "failed to decode oidc ca bundle")

		return ""
	}

	return string(decodedBytes)
}

// GetClientID returns the OIDC Provider client_id for the given workload target
func (c *OIDCAppsControllerConfig) GetClientID(object client.Object) string {
	t := c.FetchTarget(object)

	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.ClientID != "" {
		return t.Oauth2Proxy.ClientID
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.ClientID != "" {
		return c.Global.Oauth2Proxy.ClientID
	}

	return ""
}

// GetClientSecret returns the OIDC Provider secret for the given target workload
func (c *OIDCAppsControllerConfig) GetClientSecret(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.ClientSecret != "" {
		return t.Oauth2Proxy.ClientSecret
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.ClientSecret != "" {
		return c.Global.Oauth2Proxy.ClientSecret
	}

	return ""
}

// GetScope returns the OIDC Provider scope for the given target workload
func (c *OIDCAppsControllerConfig) GetScope(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.Scope != "" {
		return t.Oauth2Proxy.Scope
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.Scope != "" {
		return c.Global.Oauth2Proxy.Scope
	}

	return ""
}

// GetRedirectURL returns the OIDC Provider redirect URL for the given workload target
func (c *OIDCAppsControllerConfig) GetRedirectURL(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.RedirectURL != "" {
		return t.Oauth2Proxy.RedirectURL
	}

	// The redirect URL shall not default to the global one.
	// Instead, it shall be constructed as below code */
	// If the target oidc configuration does not define a redirect URL
	// it will be constructed as https://{name}-{namespace}.domainName/oauth2/callback
	return "https://" + c.GetHost(object) + "/oauth2/callback"
}

// GetOidcIssuerURL returns the OIDC Provider URL for the given workload target
func (c *OIDCAppsControllerConfig) GetOidcIssuerURL(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.OidcIssuerURL != "" {
		return t.Oauth2Proxy.OidcIssuerURL
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.OidcIssuerURL != "" {
		return c.Global.Oauth2Proxy.OidcIssuerURL
	}

	return ""
}

// GetSslInsecureSkipVerify designates if oauth2-proxy shall skip upstream ssl validation
func (c *OIDCAppsControllerConfig) GetSslInsecureSkipVerify(object client.Object) bool {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.SSLInsecureSkipVerify != nil {
		return ptr.Deref(t.Oauth2Proxy.SSLInsecureSkipVerify, false)
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.SSLInsecureSkipVerify != nil {
		return ptr.Deref(c.Global.Oauth2Proxy.SSLInsecureSkipVerify, false)
	}

	return false
}

// GetInsecureOidcSkipIssuerVerification designates if oauth2-proxy shall skip OIDC Provider certificate validation
func (c *OIDCAppsControllerConfig) GetInsecureOidcSkipIssuerVerification(object client.Object) bool {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.InsecureOidcSkipIssuerVerification != nil {
		return ptr.Deref(t.Oauth2Proxy.InsecureOidcSkipIssuerVerification, false)
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.InsecureOidcSkipIssuerVerification != nil {
		return ptr.Deref(c.Global.Oauth2Proxy.InsecureOidcSkipIssuerVerification, false)
	}

	return false
}

// GetInsecureOidcSkipNonce designates if oauth2-proxy shall skip OIDC nonce request parameter
func (c *OIDCAppsControllerConfig) GetInsecureOidcSkipNonce(object client.Object) bool {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.InsecureOidcSkipNonce != nil {
		return ptr.Deref(t.Oauth2Proxy.InsecureOidcSkipNonce, false)
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.InsecureOidcSkipNonce != nil {
		return ptr.Deref(c.Global.Oauth2Proxy.InsecureOidcSkipNonce, false)
	}

	return false
}

// GetApprovalPrompt returns the OIDC approval prompt mode for the given workload target
func (c *OIDCAppsControllerConfig) GetApprovalPrompt(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.ApprovalPrompt != "" {
		return t.Oauth2Proxy.ApprovalPrompt
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.ApprovalPrompt != "" {
		return c.Global.Oauth2Proxy.ApprovalPrompt
	}

	return "auto"
}

// GetCookieRefresh returns the oauth2-proxy cookie refresh interval for the given workload target
func (c *OIDCAppsControllerConfig) GetCookieRefresh(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.CookieRefresh != "" {
		return t.Oauth2Proxy.CookieRefresh
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.CookieRefresh != "" {
		return c.Global.Oauth2Proxy.CookieRefresh
	}

	return "3600s"
}

// GetEmailDomain returns the email domain restriction for the given workload target
func (c *OIDCAppsControllerConfig) GetEmailDomain(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.EmailDomain != "" {
		return t.Oauth2Proxy.EmailDomain
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.EmailDomain != "" {
		return c.Global.Oauth2Proxy.EmailDomain
	}

	return "*"
}

// GetSkipProviderButton returns whether to skip the oauth2-proxy provider selection button
func (c *OIDCAppsControllerConfig) GetSkipProviderButton(object client.Object) bool {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.SkipProviderButton != nil {
		return ptr.Deref(t.Oauth2Proxy.SkipProviderButton, true)
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.SkipProviderButton != nil {
		return ptr.Deref(c.Global.Oauth2Proxy.SkipProviderButton, true)
	}

	return true
}

// GetCodeChallengeMethod returns the PKCE code challenge method for the given workload target
func (c *OIDCAppsControllerConfig) GetCodeChallengeMethod(object client.Object) string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		t.Oauth2Proxy.CodeChallengeMethod != "" {
		return t.Oauth2Proxy.CodeChallengeMethod
	}

	if c.Global.Oauth2Proxy != nil &&
		c.Global.Oauth2Proxy.CodeChallengeMethod != "" {
		return c.Global.Oauth2Proxy.CodeChallengeMethod
	}

	return "S256"
}

// GetExtraArgs returns extra CLI arguments for oauth2-proxy for the given workload target
func (c *OIDCAppsControllerConfig) GetExtraArgs(object client.Object) []string {
	t := c.FetchTarget(object)
	if t.Oauth2Proxy != nil &&
		len(t.Oauth2Proxy.ExtraArgs) > 0 {
		return t.Oauth2Proxy.ExtraArgs
	}

	if c.Global.Oauth2Proxy != nil &&
		len(c.Global.Oauth2Proxy.ExtraArgs) > 0 {
		return c.Global.Oauth2Proxy.ExtraArgs
	}

	return nil
}
