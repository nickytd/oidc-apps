#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 nickytd
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHART_DIR="${SCRIPT_DIR}/../../charts/oidc-apps"
CLUSTER_NAME="oidc-apps"

echo "==> Creating Kind cluster..."
kind create cluster --config "${SCRIPT_DIR}/kind-config.yaml"

echo "==> Building controller image..."
make -C "${SCRIPT_DIR}/../.." docker-image-controller

echo "==> Building kube-rbac-proxy image..."
make -C "${SCRIPT_DIR}/../.." docker-image-watcher

echo "==> Loading images into Kind..."
kind load docker-image ghcr.io/nickytd/oidc-apps/oidc-apps:latest --name "${CLUSTER_NAME}"
kind load docker-image ghcr.io/nickytd/oidc-apps/kube-rbac-proxy:latest --name "${CLUSTER_NAME}"

echo "==> Installing Envoy Gateway (includes Gateway API CRDs)..."
helm install envoy-gateway oci://docker.io/envoyproxy/gateway-helm \
  --version v1.2.0 \
  --namespace envoy-gateway-system \
  --create-namespace

echo "==> Waiting for Envoy Gateway to be ready..."
kubectl wait --namespace envoy-gateway-system \
  --for=condition=Available deployment/envoy-gateway \
  --timeout=120s

echo "==> Deploying Dex..."
kubectl apply -f "${SCRIPT_DIR}/dex.yaml"

echo "==> Deploying Grafana..."
kubectl apply -f "${SCRIPT_DIR}/grafana.yaml"

echo "==> Creating self-signed TLS certificate..."
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout "${SCRIPT_DIR}/tls.key" -out "${SCRIPT_DIR}/tls.crt" \
  -subj "/CN=*.127.0.0.1.nip.io" \
  -addext "subjectAltName=DNS:*.127.0.0.1.nip.io" 2>/dev/null

OIDC_CA_BUNDLE=$(base64 < "${SCRIPT_DIR}/tls.crt" | tr -d '\n')

kubectl create secret tls wildcard-tls \
  --cert="${SCRIPT_DIR}/tls.crt" --key="${SCRIPT_DIR}/tls.key" \
  --namespace=default

rm -f "${SCRIPT_DIR}/tls.key" "${SCRIPT_DIR}/tls.crt"

echo "==> Applying Gateway resources..."
kubectl apply -f "${SCRIPT_DIR}/gateway.yaml"

echo "==> Waiting for Envoy Gateway proxy to be ready..."
echo "    Waiting for proxy pod to be created..."
until kubectl get pod -n envoy-gateway-system -l app.kubernetes.io/component=proxy -o name 2>/dev/null | grep -q pod; do
  sleep 2
done
kubectl wait --namespace envoy-gateway-system \
  --for=condition=Ready pod -l app.kubernetes.io/component=proxy \
  --timeout=120s

echo "==> Patching gateway service to use NodePort 30443..."
GATEWAY_SVC=$(kubectl get svc -n envoy-gateway-system -l app.kubernetes.io/component=proxy -o name)
kubectl patch -n envoy-gateway-system "${GATEWAY_SVC}" --type='json' \
  -p='[{"op": "replace", "path": "/spec/ports/0/nodePort", "value": 30443}]'

echo "==> Patching CoreDNS to resolve *.127.0.0.1.nip.io in-cluster..."
GATEWAY_IP=$(kubectl get "${GATEWAY_SVC}" -n envoy-gateway-system -o jsonpath='{.spec.clusterIP}')
COREFILE=$(cat <<EOF
.:53 {
    errors
    health {
       lameduck 5s
    }
    ready
    hosts {
       ${GATEWAY_IP} dex.127.0.0.1.nip.io grafana-monitoring.127.0.0.1.nip.io
       fallthrough
    }
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30 {
       disable success cluster.local
       disable denial cluster.local
    }
    loop
    reload
    loadbalance
}
EOF
)
kubectl create configmap coredns -n kube-system \
  --from-literal="Corefile=${COREFILE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment/coredns -n kube-system
kubectl rollout status deployment/coredns -n kube-system --timeout=60s

echo "==> Installing oidc-apps-controller..."
sed "s|OIDC_CA_BUNDLE_PLACEHOLDER|${OIDC_CA_BUNDLE}|" "${SCRIPT_DIR}/oidc-apps-config.yaml" > "${SCRIPT_DIR}/oidc-apps-config-resolved.yaml"
helm install oidc-apps "${CHART_DIR}" \
  --namespace oidc-apps \
  --create-namespace \
  --set fullnameOverride=oidc-apps \
  -f "${SCRIPT_DIR}/oidc-apps-config-resolved.yaml"
rm -f "${SCRIPT_DIR}/oidc-apps-config-resolved.yaml"

echo "==> Applying RBAC..."
kubectl apply -f "${SCRIPT_DIR}/rbac.yaml"

echo "==> Waiting for oidc-apps controller to be ready..."
kubectl wait --namespace oidc-apps \
  --for=condition=Available deployment/oidc-apps \
  --timeout=120s

echo "==> Rolling restart of Grafana..."
kubectl rollout restart deployment/grafana --namespace monitoring
kubectl rollout status deployment/grafana --namespace monitoring --timeout=120s

echo ""
echo "Setup complete! Access Grafana at:"
echo "  https://grafana-monitoring.127.0.0.1.nip.io"
echo ""
echo "Login credentials:"
echo "  Email:    user@oidc-apps.io"
echo "  Password: password"
echo ""
echo "Cleanup: kind delete cluster --name ${CLUSTER_NAME}"
