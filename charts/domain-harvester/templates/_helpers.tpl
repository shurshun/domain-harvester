{{/*
Expand the name of the chart.
*/}}
{{- define "domain-harvester.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "domain-harvester.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "domain-harvester.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "domain-harvester.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "domain-harvester.selectorLabels" -}}
app.kubernetes.io/name: {{ include "domain-harvester.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Name of the service account to use.
*/}}
{{- define "domain-harvester.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ default (include "domain-harvester.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
{{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{/*
RBAC rules for the optional CRD-backed domain sources, gated per-source so a
release that enables none of them doesn't request permissions it never uses.
Shared between the ClusterRole and Role branches of rbac.yaml.
*/}}
{{- define "domain-harvester.optionalSourceRules" -}}
{{- if .Values.sources.traefikIngressRoute.enabled }}
- apiGroups: ["traefik.io"]
  resources: ["ingressroutes"]
  verbs: ["get", "list", "watch"]
{{- end }}
{{- if .Values.sources.gatewayHTTPRoute.enabled }}
- apiGroups: ["gateway.networking.k8s.io"]
  resources: ["httproutes"]
  verbs: ["get", "list", "watch"]
{{- end }}
{{- if .Values.sources.gatewayGRPCRoute.enabled }}
- apiGroups: ["gateway.networking.k8s.io"]
  resources: ["grpcroutes"]
  verbs: ["get", "list", "watch"]
{{- end }}
{{- end -}}
