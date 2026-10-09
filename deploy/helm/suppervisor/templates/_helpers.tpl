{{- define "suppervisor.name" -}}suppervisor{{- end }}
{{- define "suppervisor.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "suppervisor.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end }}
{{- define "suppervisor.labels" -}}
app.kubernetes.io/name: {{ include "suppervisor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | quote }}
{{- end }}
{{- define "suppervisor.selectorLabels" -}}
app.kubernetes.io/name: {{ include "suppervisor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
{{- define "suppervisor.securityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: true
runAsNonRoot: true
capabilities:
  drop: ["ALL"]
seccompProfile:
  type: RuntimeDefault
{{- end }}
