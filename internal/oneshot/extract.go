package oneshot

import (
	"regexp"
	"strings"
)

var (
	fencedHTML = regexp.MustCompile("(?s)```(?:html)?\\s*\n(.*?)\n```")
	rawHTML    = regexp.MustCompile("(?is)(<!doctype\\s+html|<html\\b).*?(</html>)")
)

func ExtractHTML(output string) (string, bool) {
	if m := fencedHTML.FindStringSubmatch(output); len(m) > 1 {
		html := strings.TrimSpace(m[1])
		if looksLikeHTML(html) {
			return html, true
		}
	}

	if m := rawHTML.FindString(output); m != "" {
		return strings.TrimSpace(m), true
	}

	return "", false
}

func looksLikeHTML(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "<!doctype") ||
		strings.Contains(lower, "<head") ||
		strings.Contains(lower, "<body")
}
