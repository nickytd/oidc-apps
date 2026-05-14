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

func mutateOauth2Secret(secret *corev1.Secret, object client.Object) error {
	extConfig := configuration.GetOIDCAppsControllerConfig()

	var cfg string

	switch extConfig.GetClientSecret(object) {
	case "":
		cfg = configuration.NewOAuth2Config(
			configuration.WithClientID(extConfig.GetClientID(object)),
			configuration.WithClientSecretFile("/dev/null"),
			configuration.WithScope(extConfig.GetScope(object)),
			configuration.WithRedirectURL(extConfig.GetRedirectURL(object)),
			configuration.WithOidcIssuerURL(extConfig.GetOidcIssuerURL(object)),
			configuration.EnableSslInsecureSkipVerify(extConfig.GetSslInsecureSkipVerify(object)),
			configuration.EnableInsecureOidcSkipIssuerVerification(extConfig.GetInsecureOidcSkipIssuerVerification(object)),
			configuration.EnableInsecureOidcSkipNonce(extConfig.GetInsecureOidcSkipNonce(object)),
		).Parse()
	default:
		cfg = configuration.NewOAuth2Config(
			configuration.WithClientID(extConfig.GetClientID(object)),
			configuration.WithClientSecret(extConfig.GetClientSecret(object)),
			configuration.WithScope(extConfig.GetScope(object)),
			configuration.WithRedirectURL(extConfig.GetRedirectURL(object)),
			configuration.WithOidcIssuerURL(extConfig.GetOidcIssuerURL(object)),
			configuration.EnableSslInsecureSkipVerify(extConfig.GetSslInsecureSkipVerify(object)),
			configuration.EnableInsecureOidcSkipIssuerVerification(extConfig.GetInsecureOidcSkipIssuerVerification(object)),
			configuration.EnableInsecureOidcSkipNonce(extConfig.GetInsecureOidcSkipNonce(object)),
		).Parse()
	}

	checksum := randutils.GenerateFullSha256(cfg)

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.Oauth2LabelValue,
	}
	secret.Annotations = map[string]string{constants.AnnotationOauth2SecretChecksumKey: checksum}
	secret.Data = map[string][]byte{"oauth2-proxy.cfg": []byte(cfg)}

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

func mutateKubeconfigSecret(secret *corev1.Secret, object client.Object) error {
	kubeConfigStr := configuration.GetOIDCAppsControllerConfig().GetKubeConfigStr(object)

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

func mutateOidcCaBundleSecret(secret *corev1.Secret, object client.Object) error {
	oidcCABundle := configuration.GetOIDCAppsControllerConfig().GetOidcCABundle(object)

	secret.Labels = map[string]string{
		constants.LabelKey:       constants.LabelValue,
		constants.SecretLabelKey: constants.OidcCa2LabelValue,
	}
	secret.StringData = map[string]string{"ca.crt": oidcCABundle}

	return nil
}

func needsKubeconfigSecret(object client.Object) bool {
	return configuration.GetOIDCAppsControllerConfig().GetKubeConfigStr(object) != ""
}

func needsOidcCaBundleSecret(object client.Object) bool {
	return configuration.GetOIDCAppsControllerConfig().GetOidcCABundle(object) != ""
}
