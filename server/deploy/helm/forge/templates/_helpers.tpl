{{- define "velora.name" -}}velora{{- end -}}
{{- define "velora.fullname" -}}{{ .Release.Name }}-velora{{- end -}}
{{- define "velora.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{ include "velora.fullname" . }}
{{- else -}}
{{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}
{{- define "velora.secretName" -}}
{{- if .Values.secretEnv.existingSecret -}}
{{ .Values.secretEnv.existingSecret }}
{{- else -}}
{{ include "velora.fullname" . }}
{{- end -}}
{{- end -}}
{{- define "velora.selectorLabels" -}}
app.kubernetes.io/name: {{ include "velora.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
{{- define "velora.image" -}}
{{- $repository := required "image.repository is required" .Values.image.repository -}}
{{- $digest := default "" .Values.image.digest -}}
{{- $environment := lower (default "development" .Values.config.environment) -}}
{{- $production := or (eq $environment "production") (eq $environment "prod") -}}
{{- if $digest -}}
  {{- if not (regexMatch "^sha256:[a-f0-9]{64}$" $digest) -}}
    {{- fail "image.digest must be a lowercase sha256 digest" -}}
  {{- end -}}
{{- else if $production -}}
  {{- fail "image.digest is required in production; mutable tags are forbidden" -}}
{{- end -}}
{{- if $digest -}}
{{- printf "%s@%s" $repository $digest -}}
{{- else -}}
{{- printf "%s:%s" $repository (required "image.tag is required when image.digest is empty" .Values.image.tag) -}}
{{- end -}}
{{- end -}}
