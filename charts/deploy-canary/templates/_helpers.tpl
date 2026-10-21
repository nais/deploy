{{/*
Expand the name of the chart.
*/}}
{{- define "deploycanary.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "deploycanary.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "deploycanary.labels" -}}
app: {{ include "deploycanary.name" . }}
helm.sh/chart: {{ include "deploycanary.chart" . }}
{{ include "deploycanary.selectorLabels" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "deploycanary.selectorLabels" -}}
app.kubernetes.io/name: {{ include "deploycanary.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Shared pod spec for the canary binary. Pass a dict with "ctx" (root context)
and "command" (provision|deploy).
*/}}
{{- define "deploycanary.podspec" -}}
{{- $ctx := .ctx -}}
restartPolicy: Never
{{- with $ctx.Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
securityContext:
  seccompProfile:
    type: RuntimeDefault
  runAsUser: 65534
  runAsNonRoot: true
  runAsGroup: 65534
  fsGroup: 65534
containers:
  - name: canary
    image: "{{ $ctx.Values.image.repository }}:{{ $ctx.Values.image.tag }}"
    imagePullPolicy: {{ $ctx.Values.image.pullPolicy }}
    args:
      - {{ .command | quote }}
    securityContext:
      capabilities:
        drop:
          - ALL
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
    env:
      - name: GRPC_TARGET
        value: {{ $ctx.Values.grpcTarget | quote }}
      - name: API_BASE_URL
        value: {{ $ctx.Values.apiBaseUrl | quote }}
      - name: ENVIRONMENTS
        value: {{ join "," $ctx.Values.environments | quote }}
{{- end }}
