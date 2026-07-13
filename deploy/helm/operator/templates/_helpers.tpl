{{/*
Chart fullname, following the standard Helm chart convention.
*/}}
{{- define "queueworker-operator.fullname" -}}
{{- if contains .Chart.Name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Common labels applied to every resource this chart creates.
*/}}
{{- define "queueworker-operator.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Selector labels for the operator Deployment.
*/}}
{{- define "queueworker-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
In-cluster Redis address for the bundled Bitnami subchart, standalone
architecture. Bitnami's chart name is "redis", so its fullname (absent a
nameOverride) is "<release-name>-redis", and the standalone master Service
is "<fullname>-master".
*/}}
{{- define "queueworker-operator.bundledRedisAddress" -}}
{{- printf "%s-redis-master:6379" .Release.Name }}
{{- end }}
