package scanmatch

import "strings"

type Replacement struct {
	Old string
	New string
}

func NormalizeText(value string, replacements ...Replacement) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.ReplaceAll(value, "_", " ")
	for _, replacement := range replacements {
		value = strings.ReplaceAll(value, replacement.Old, replacement.New)
	}
	for strings.Contains(value, "  ") {
		value = strings.ReplaceAll(value, "  ", " ")
	}
	return strings.TrimSpace(value)
}
