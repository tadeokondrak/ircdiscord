package client

import (
	"strings"
	"unicode"
)

func ircUsername(s string) string {
	return strings.Map(
		func(r rune) rune {
			if unicode.IsLetter(r) ||
				unicode.IsNumber(r) {
				return r
			}
			switch r {
			case '_', '-', '{', '}', '[', ']', '\\', '`', '|':
				return r
			}
			return -1
		},
		s)
}
