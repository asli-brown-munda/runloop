package secrets

func Redact(value string) string {
	if value == "" {
		return ""
	}
	return "[REDACTED]"
}
