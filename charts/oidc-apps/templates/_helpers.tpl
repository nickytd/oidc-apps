# SPDX-FileCopyrightText: 2026 nickytd
# SPDX-License-Identifier: Apache-2.0
---
{{/*
Expand the name of the chart.
*/}}
{{- define "oidc-apps.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "oidc-apps.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "oidc-apps.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "oidc-apps.labels" -}}
helm.sh/chart: {{ include "oidc-apps.chart" . }}
{{ include "oidc-apps.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "oidc-apps.selectorLabels" -}}
app.kubernetes.io/name: {{ include "oidc-apps.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Certificate labels
*/}}
{{- define "oidc-apps.certificateLabels" -}}
{{ include "oidc-apps.labels" . }}
app.kubernetes.io/component: certificate
{{- end }}

{{/*
Create the name of the certificate secret to use
*/}}
{{- define "oidc-apps.certificateSecretName" -}}
{{- if .Values.certificate.create }}
{{- $defaultSecretName :=  printf "%s-%s" (include "oidc-apps.fullname" .) "webhook-tls-cert" -}}
{{- default $defaultSecretName .Values.certificate.secretName }}
{{- else }}
{{- default "default" "" }}
{{- end }}
{{- end }}

{{/*
Create the certificate reference for the ca-injector
*/}}
{{- define "oidc-apps.certificateRef" -}}
{{- printf "%s/%s" .Release.Namespace (include "oidc-apps.fullname" .) -}}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "oidc-apps.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "oidc-apps.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Create the name of the cluster role to use
*/}}
{{- define "oidc-apps.clusterRoleName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "oidc-apps.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{-  define "image" -}}
  {{- if hasPrefix "sha256:" .Values.image.tag }}
  {{- printf "%s@%s" .Values.image.repository .Values.image.tag }}
  {{- else }}
  {{- printf "%s:%s" .Values.image.repository .Values.image.tag }}
  {{- end }}
{{- end }}
