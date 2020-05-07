package ircdiscord

import (
	"strings"
	"unicode"
)

func ircUsername(s string) string {
	var b strings.Builder
	for _, c := range s {
		if !unicode.IsSpace(c) && unicode.IsGraphic(c) {
			b.WriteRune(c)
		}
	}
	return b.String()
}
