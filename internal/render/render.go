package render

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/ningen/md"
	"github.com/sourcegraph/syntaxhighlight"
	"github.com/tadeokondrak/ircdiscord/internal/color"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"github.com/yuin/goldmark/ast"
)

func Content(guildID discord.Snowflake, sess *session.Session, source []byte, m *discord.Message) string {
	parsed := md.ParseWithMessage(source, sess.Store, m, false)
	var s strings.Builder
	var walker func(n ast.Node, enter bool) (ast.WalkStatus, error)
	walker = func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Document:
			// intentionally left blank
		case *ast.Blockquote:
			if enter {
				for child := n.FirstChild(); child != nil; child = child.NextSibling() {
					s.WriteString("\x0309>\x03 ")
					ast.Walk(child, func(node ast.Node, enter bool) (ast.WalkStatus, error) {
						// We only call when entering, since we don't want to trigger a
						// hard new line after each paragraph.
						if enter {
							walker(node, true)
						}
						return ast.WalkContinue, nil
					})
				}
				return ast.WalkSkipChildren, nil
			}
		case *ast.Paragraph:
			if !enter {
				if m != nil && m.EditedTimestamp.Valid() {
					s.WriteString(" \x1D\x0311(edited)\x03\x1D")
				}
				s.WriteString("\n")
			}
		case *ast.FencedCodeBlock:
			if enter {
				var content bytes.Buffer
				for i := 0; i < n.Lines().Len(); i++ {
					line := n.Lines().At(i)
					if i == 0 && len(line.Value(source)) == 0 {
						continue
					}
					content.Write(line.Value(source))

				}
				scanner := syntaxhighlight.NewScanner(content.Bytes())
				var highlighted strings.Builder
				syntaxhighlight.Print(scanner, &highlighted, ircPrinter{})
				for i, line := range strings.Split(strings.Trim(highlighted.String(), "\n"), "\n") {
					if i == 0 && line == "" {
						continue
					}
					s.WriteString("\x0314>\x03 ")
					s.WriteString(line)
					s.WriteString("\n")
				}
			}
		case *ast.Link:
			if enter {
				fmt.Fprintf(&s, "\x0302[\x03")
			} else {
				fmt.Fprintf(&s, " \x0302%s]\x03", n.Destination)
			}
		case *ast.AutoLink:
			if enter {
				fmt.Fprintf(&s, "\x0302%s\x03", n.URL(source))
			}
		case *md.Inline:
			switch n.Attr {
			case md.AttrBold:
				s.WriteByte(0x02)
			case md.AttrItalics:
				s.WriteByte(0x1D)
			case md.AttrUnderline:
				s.WriteByte(0x1F)
			case md.AttrStrikethrough:
				s.WriteByte(0x1E)
			case md.AttrSpoiler:
				if enter {
					s.WriteString("\x0300,00")
				} else {
					s.WriteString("\x03")
				}
			case md.AttrMonospace:
				if enter {
					s.WriteString("\x0314")
				} else {
					s.WriteString("\x03")
				}
			case md.AttrQuoted:
				// not sure what this is
			}
		case *md.Emoji:
			if enter {
				fmt.Fprintf(&s, "\x0303:%s:\x03", n.Name)
			}
		case *md.Mention:
			if enter {
				switch {
				case n.Channel != nil:
					fmt.Fprintf(&s, "\x02\x0302#%s\x03\x02", n.Channel.Name)
				case n.GuildUser != nil:
					name, err := sess.UserName(guildID, n.GuildUser.User.ID)
					if err != nil {
						name = n.GuildUser.User.Username
					}
					fmt.Fprintf(&s, "\x02\x0302@%s\x03\x02", name)
				}
			}
		case *ast.String:
			if enter {
				s.Write(md.Unescape(n.Value))
			}
		case *ast.Text:
			if enter {
				s.Write(md.Unescape(n.Segment.Value(source)))
				switch {
				case n.HardLineBreak():
					s.WriteString("\n\n")
				case n.SoftLineBreak():
					s.WriteString("\n")
				}
			}
		}
		return ast.WalkContinue, nil
	}
	ast.Walk(parsed, walker)
	return s.String()
}

func Message(guildID discord.Snowflake, sess *session.Session, m *discord.Message) (string, error) {
	if m.Type != discord.DefaultMessage {
		return "", nil
	}
	var s strings.Builder
	s.WriteString(Content(guildID, sess, []byte(m.Content), m))
	for _, e := range m.Embeds {
		var es strings.Builder
		if e.Title != "" {
			fmt.Fprintf(&es, "\x02%s\x02", e.Title)
			if e.URL != "" {
				fmt.Fprintf(&es, " \x0302%s\x03", e.URL)
			}
			es.WriteString("\n")
		}

		if e.Description != "" {
			es.WriteString(Content(guildID, sess, []byte(e.Description), m))
			es.WriteString("\n")
		}

		for _, f := range e.Fields {
			fmt.Fprintf(&es, "\x1D%s:\x1D ", f.Name)
			if !f.Inline {
				es.WriteString("\n")
			}
			es.WriteString(Content(guildID, sess, []byte(f.Value), m))
			es.WriteString("\n")
		}
		embed := strings.Split(strings.Trim(es.String(), "\n"), "\n")
		for i, line := range embed {
			if i == 0 && line == "" {
				continue
			}
			fmt.Fprintf(&s, "\x03%d▌\x03\x02\x02", color.Nearest(e.Color.Uint32()))
			s.WriteString(line)
			s.WriteString("\n")
		}
	}
	for _, a := range m.Attachments {
		fmt.Fprintf(&s, "\x02%s\x02 (size: %d", a.Filename, a.Size)
		if a.Width != 0 && a.Height != 0 {
			fmt.Fprintf(&s, ", %dx%d", a.Width, a.Height)
		}
		fmt.Fprintf(&s, "): \x0302%s\x03", a.URL)
		if a.Proxy != strings.Replace(a.URL, "cdn.discordapp.com", "media.discordapp.net", 1) {
			fmt.Fprintf(&s, " | \x0302%s\x03\n", a.Proxy)
		}
	}
	return strings.Trim(s.String(), "\n"), nil

}

type ircPrinter struct{}

func (ircPrinter) Print(w io.Writer, kind syntaxhighlight.Kind, tokText string) error {
	// we ignore errors since we're always printing into a buffer
	switch kind {
	case syntaxhighlight.String:
		io.WriteString(w, "\x0309")
	case syntaxhighlight.Keyword:
		io.WriteString(w, "\x0304")
	case syntaxhighlight.Comment:
		io.WriteString(w, "\x0314")
	case syntaxhighlight.Type:
		io.WriteString(w, "\x0310")
	case syntaxhighlight.Literal:
		io.WriteString(w, "\x0307")
	case syntaxhighlight.Punctuation:
		io.WriteString(w, "\x0308")
	case syntaxhighlight.Plaintext:
		io.WriteString(w, "\x0300")
	case syntaxhighlight.Tag:
		io.WriteString(w, "\x0300")
	case syntaxhighlight.HTMLTag:
		io.WriteString(w, "\x0304")
	case syntaxhighlight.HTMLAttrName:
		io.WriteString(w, "\x0300")
	case syntaxhighlight.HTMLAttrValue:
		io.WriteString(w, "\x0309")
	case syntaxhighlight.Decimal:
		io.WriteString(w, "\x0307")
	default:
		io.WriteString(w, "\x0300")
	}
	io.WriteString(w, "\x02\x02")
	io.WriteString(w, tokText)
	io.WriteString(w, "\x03")
	return nil
}
