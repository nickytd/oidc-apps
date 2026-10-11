// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SecretNameOauth2Proxy + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
	}
}

func mutateOauth2Secret(secret *corev1.Secret, rc *configuration.ResolvedConfig) error {
	var oauth2Cfg string

	switch rc.GetClientSecret() {
	case "":
		oauth2Cfg = configuration.NewOAuth2Config(
			configuration.WithClientID(rc.GetClientID()),
			configuration.WithClientSecretFile("/dev/null"),
			configuration.WithScope(rc.GetScope()),
			configuration.WithRedirectURL(rc.GetRedirectURL()),
			configuration.WithOidcIssuerURL(rc.GetOidcIssuerURL()),
			configuration.EnableSslInsecureSkipVerify(rc.GetSslInsecureSkipVerify()),
			configuration.EnableInsecureOidcSkipIssuerVerification(rc.GetInsecureOidcSkipIssuerVerification()),
			configuration.EnableInsecureOidcSkipNonce(rc.GetInsecureOidcSkipNonce()),
		).Parse()
	default:
		oauth2Cfg = configuration.NewOAuth2Config(
			configuration.WithClientID(rc.GetClientID()),
			configuration.WithClientSecret(rc.GetClientSecret()),
			configuration.WithScope(rc.GetScope()),
			configuration.WithRedirectURL(rc.GetRedirectURL()),
			configuration.WithOidcIssuerURL(rc.GetOidcIssuerURL()),
			configuration.EnableSslInsecureSkipVerify(rc.GetSslInsecureSkipVerify()),
			configuration.EnableInsecureOidcSkipIssuerVerification(rc.GetInsecureOidcSkipIssuerVerification()),
			configuration.EnableInsecureOidcSkipNonce(rc.GetInsecureOidcSkipNonce()),
		).Parse()
	}

	checksum := randutils.GenerateFullSha256(oauth2Cfg)

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.Oauth2LabelValue,
	}
	secret.Annotations = map[string]string{constants.AnnotationOauth2SecretChecksumKey: checksum}
	secret.Data = map[string][]byte{"oauth2-proxy.cfg": []byte(oauth2Cfg)}

	return nil
}

func resourceAttributesSecretObject(object client.Object) *corev1.Secret {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SecretNameResourceAttributes + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SecretNameKubeconfig + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SecretNameOidcCa + "-" + suffix,
			Namespace: object.GetNamespace(),
		},
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
