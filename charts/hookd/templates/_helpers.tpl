{{- define "hookd.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "hookd.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "hookd.labels" -}}
helm.sh/chart: {{ include "hookd.chart" . }}
{{ include "hookd.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "hookd.selectorLabels" -}}
app.kubernetes.io/name: {{ include "hookd.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
