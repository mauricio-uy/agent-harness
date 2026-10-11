package docs

import (
	"net/url"
	"regexp"
	"strings"
)

// Keep a whole URL in one token so a local-looking path inside its query,
// fragment or parenthesized path cannot be mistaken for a project reference.
var referenceToken = regexp.MustCompile("[^\\s\"'\\x60<>]+")

func externalReferenceSpans(text string) [][2]int {
	var spans [][2]int
	for _, span := range referenceToken.FindAllStringIndex(text, -1) {
		token := text[span[0]:span[1]]
		start := span[0] + len(token) - len(strings.TrimLeft(token, "([{"))
		end := span[0] + len(strings.TrimRight(token, ".,;:!)]}"))
		if start >= end {
			continue
		}
		candidate := text[start:end]
		if key, value, ok := strings.Cut(candidate, "="); ok && !strings.ContainsAny(key, ":/?#") {
			start += len(key) + 1
			candidate = value
		}
		parsed, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if parsed.IsAbs() || parsed.Host != "" {
			spans = append(spans, [2]int{start, end})
		}
	}
	return spans
}
