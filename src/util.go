package ircdiscord

import (
	"strings"
	"unicode"
)

func ircClean(s string) string {
	var b strings.Builder
	for _, c := range s {
		if !unicode.IsSpace(c) && unicode.IsGraphic(c) {
			b.WriteRune(c)
		}
	}
	return b.String()
}
