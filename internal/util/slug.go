package util

import (
	"strings"
	"unicode"
)

func SlugTR(value string) string {
	replacer := strings.NewReplacer(
		"İ", "I",
		"I", "i",
		"ı", "i",
		"Ö", "O",
		"ö", "o",
		"Ü", "U",
		"ü", "u",
		"Ğ", "G",
		"ğ", "g",
		"Ş", "S",
		"ş", "s",
		"Ç", "C",
		"ç", "c",
	)
	value = strings.ToLower(replacer.Replace(value))

	var b strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
