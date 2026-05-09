// SPDX-FileCopyrightText: 2026 nickytd
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"encoding/base64"
	"errors"
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

var errSecretDoesNotExist = errors.New("secret does not exist")

func createOauth2Secret(object client.Object) (corev1.Secret, error) {
	var cfg string

	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())
	extConfig := configuration.GetOIDCAppsControllerConfig()

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
			configuration.EnableInsecureOidcSkipNonce(extConfig.GetInsecureOidcSkipNonce(object))).Parse()

	default:
		cfg = configuration.NewOAuth2Config(
			configuration.WithClientID(extConfig.GetClientID(object)),
			configuration.WithClientSecret(extConfig.GetClientSecret(object)),
			configuration.WithScope(extConfig.GetScope(object)),
			configuration.WithRedirectURL(extConfig.GetRedirectURL(object)),
			configuration.WithOidcIssuerURL(extConfig.GetOidcIssuerURL(object)),
			configuration.EnableSslInsecureSkipVerify(extConfig.GetSslInsecureSkipVerify(object)),
			configuration.EnableInsecureOidcSkipIssuerVerification(extConfig.GetInsecureOidcSkipIssuerVerification(object)),
			configuration.EnableInsecureOidcSkipNonce(extConfig.GetInsecureOidcSkipNonce(object))).Parse()
	}

	checksum := randutils.GenerateFullSha256(cfg)

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        constants.SecretNameOauth2Proxy + "-" + suffix,
			Namespace:   object.GetNamespace(),
			Annotations: map[string]string{constants.AnnotationOauth2SecertCehcksumKey: checksum},
			Labels: map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.Oauth2LabelValue,
			},
		},
		Data: map[string][]byte{"oauth2-proxy.cfg": []byte(cfg)},
	}, nil
}

func createResourceAttributesSecret(object client.Object, targetNamespace string) (corev1.Secret, error) {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	// TODO: add configurable resource, subresource
	cfg := configuration.NewResourceAttributes(
		configuration.WithNamespace(targetNamespace),
		configuration.WithSubresource(object.GetName()),
	).Parse()

	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SecretNameResourceAttributes + "-" + suffix,
			Namespace: object.GetNamespace(),
			Labels: map[string]string{
				constants.LabelKey:       constants.LabelValue,
				constants.SecretLabelKey: constants.RbacLabelValue,
			},
		},
		StringData: map[string]string{"config-file.yaml": cfg},
	}, nil
}

func createKubeconfigSecret(object client.Object) (corev1.Secret, error) {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	kubeConfigStr := configuration.GetOIDCAppsControllerConfig().GetKubeConfigStr(object)
	if len(kubeConfigStr) > 0 {
		decodestr, err := base64.StdEncoding.DecodeString(kubeConfigStr)
		if err != nil {
			return corev1.Secret{}, fmt.Errorf("kubeconfig is not base64 encoded: %w", err)
		}

		kubeConfig := clientcmdv1.Config{}
		if err = yaml.Unmarshal(decodestr, &kubeConfig); err != nil {
			return corev1.Secret{}, fmt.Errorf("kubeconfig %s, is not in the expected format: %w", decodestr, err)
		}

		kubeconfig, _ := yaml.Marshal(kubeConfig)

		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.SecretNameKubeconfig + "-" + suffix,
				Namespace: object.GetNamespace(),
				Labels: map[string]string{
					constants.LabelKey:       constants.LabelValue,
					constants.SecretLabelKey: constants.KubeconfigLabelValue,
				},
			},
			StringData: map[string]string{"kubeconfig": string(kubeconfig)},
		}

		return secret, nil
	}

	return corev1.Secret{}, errSecretDoesNotExist
}

func createOidcCaBundleSecret(object client.Object) (corev1.Secret, error) {
	suffix := randutils.GenerateSha256(object.GetName() + "-" + object.GetNamespace())

	oidcCABundle := configuration.GetOIDCAppsControllerConfig().GetOidcCABundle(object)
	if len(oidcCABundle) > 0 {
		// TODO: verify the oidcCABundle str, it shall be CA certificates in PEM format
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.SecretNameOidcCa + "-" + suffix,
				Namespace: object.GetNamespace(),
				Labels: map[string]string{
					constants.LabelKey:       constants.LabelValue,
					constants.SecretLabelKey: constants.OidcCa2LabelValue,
				},
			},
			StringData: map[string]string{"ca.crt": oidcCABundle},
		}

		return secret, nil
	}

	return corev1.Secret{}, errSecretDoesNotExist
}
