package utils

import "strings"

// ValidateDocPath checks that a document path is safe.
func ValidateDocPath(p string) bool {
	if p == "" {
		return false
	}
	if strings.Contains(p, "..") || strings.HasPrefix(p, "/") {
		return false
	}
	allowed := []string{".md", ".txt", ".rst"}
	for _, ext := range allowed {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// SanitizeFilename removes dangerous characters from a filename.
func SanitizeFilename(name string) string {
	for _, c := range []string{"<", ">", ":", "\"", "|", "?", "*"} {
		name = strings.ReplaceAll(name, c, "_")
	}
	return strings.TrimSpace(name)
}

// TruncateText truncates text to maxLen characters with ellipsis.
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen < 3 {
		return text[:maxLen]
	}
	return text[:maxLen-3] + "..."
}
