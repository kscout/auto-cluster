{{/*
Resource name
*/}}
{{- define "auto-cluster.resource-name" -}}
{{ .Values.global.env }}-{{ .Values.global.app }}
{{- end -}}

{{/*
Resource labels
*/}}
{{- define "auto-cluster.resource-labels" -}}
app: {{ .Values.global.app }}
env: {{ .Values.global.env }}
{{- end -}}
