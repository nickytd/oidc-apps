// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/nickytd/oidc-apps/pkg/configuration"
	"github.com/nickytd/oidc-apps/pkg/constants"
	"github.com/nickytd/oidc-apps/pkg/randutils"
)

func oauth2SecretObject(object client.Object) *corev1.Secret {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &corev1.Secret{
		Name:      constants.SecretNameOauth2Proxy + "-" + suffix,
		Namespace: object.GetNamespace(),
	}
}

func mutateOauth2Secret(secret *corev1.Secret, rc *configuration.ResolvedConfig) error {
	// Choose the client-secret source in priority order: a clientSecretRef mounts the value into
	// oauth2-proxy and is referenced via client_secret_file (the value is never inlined here); an
	// inline clientSecret is written into the config; otherwise /dev/null selects PKCE.
	opts := []configuration.OptOauth2{
		configuration.WithClientID(rc.GetClientID()),
		configuration.WithScope(rc.GetScope()),
		configuration.WithRedirectURL(rc.GetRedirectURL()),
		configuration.WithOidcIssuerURL(rc.GetOidcIssuerURL()),
		configuration.EnableSslInsecureSkipVerify(rc.GetSslInsecureSkipVerify()),
		configuration.EnableInsecureOidcSkipIssuerVerification(rc.GetInsecureOidcSkipIssuerVerification()),
		configuration.EnableInsecureOidcSkipNonce(rc.GetInsecureOidcSkipNonce()),
	}

	switch {
	case rc.GetClientSecretName() != "":
		opts = append(opts, configuration.WithClientSecretFile(constants.Oauth2ClientSecretFile))
	case rc.GetClientSecret() != "":
		opts = append(opts, configuration.WithClientSecret(rc.GetClientSecret()))
	default:
		opts = append(opts, configuration.WithClientSecretFile("/dev/null"))
	}

	oauth2Cfg := configuration.NewOAuth2Config(opts...).Parse()

	checksum := randutils.GenerateFullSha256(oauth2Cfg)

	// Generate a random cookie secret once and persist it in the oauth2 secret, reusing it on
	// subsequent reconciles. This keeps the value stable across pod updates (e.g. VPA in-place
	// modifications, which forbid pod-spec changes) without deriving it from the owner identity.
	cookieSecret := secret.Data[constants.Oauth2CookieSecretKey]
	if len(cookieSecret) == 0 {
		cookieSecret = []byte(randutils.GenerateRandomString(32))
	}

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.Oauth2LabelValue,
	}
	secret.Annotations = map[string]string{constants.AnnotationOauth2SecretChecksumKey: checksum}
	secret.Data = map[string][]byte{
		"oauth2-proxy.cfg":              []byte(oauth2Cfg),
		constants.Oauth2CookieSecretKey: cookieSecret,
	}

	return nil
}

func resourceAttributesSecretObject(object client.Object) *corev1.Secret {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &corev1.Secret{
		Name:      constants.SecretNameResourceAttributes + "-" + suffix,
		Namespace: object.GetNamespace(),
	}
}

func mutateResourceAttributesSecret(secret *corev1.Secret, object client.Object, targetNamespace string) error {
	cfg := configuration.NewResourceAttributes(
		configuration.WithNamespace(targetNamespace),
		configuration.WithSubresource(object.GetName()),
	).Parse()

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.RbacLabelValue,
	}
	secret.StringData = map[string]string{"config-file.yaml": cfg}

	return nil
}

func kubeconfigSecretObject(object client.Object) *corev1.Secret {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &corev1.Secret{
		Name:      constants.SecretNameKubeconfig + "-" + suffix,
		Namespace: object.GetNamespace(),
	}
}

func mutateKubeconfigSecret(secret *corev1.Secret, rc *configuration.ResolvedConfig) error {
	kubeConfigStr := rc.GetKubeConfigStr()

	decodestr, err := base64.StdEncoding.DecodeString(kubeConfigStr)
	if err != nil {
		return fmt.Errorf("kubeconfig is not base64 encoded: %w", err)
	}

	kubeConfig := clientcmdv1.Config{}
	if err = yaml.Unmarshal(decodestr, &kubeConfig); err != nil {
		return fmt.Errorf("kubeconfig %s, is not in the expected format: %w", decodestr, err)
	}

	kubeconfig, _ := yaml.Marshal(kubeConfig)

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.KubeconfigLabelValue,
	}
	secret.StringData = map[string]string{"kubeconfig": string(kubeconfig)}

	return nil
}

func oidcCaBundleSecretObject(object client.Object) *corev1.Secret {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &corev1.Secret{
		Name:      constants.SecretNameOidcCa + "-" + suffix,
		Namespace: object.GetNamespace(),
	}
}

func mutateOidcCaBundleSecret(secret *corev1.Secret, rc *configuration.ResolvedConfig) error {
	oidcCABundle := rc.GetOidcCABundle()

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.OidcCa2LabelValue,
	}
	secret.StringData = map[string]string{"ca.crt": oidcCABundle}

	return nil
}

func needsKubeconfigSecret(rc *configuration.ResolvedConfig) bool {
	return rc.GetKubeConfigStr() != ""
}

func needsOidcCaBundleSecret(rc *configuration.ResolvedConfig) bool {
	return rc.GetOidcCABundle() != ""
}
