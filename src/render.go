package ircdiscord

import (
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/ningen/md"
	"github.com/yuin/goldmark/ast"
)

func (c *Client) renderContent(source []byte, m *discord.Message) string {
	parsed := md.ParseWithMessage(source, c.session.Store, m, true)
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
				s.WriteString("\n")
			}
		case *ast.FencedCodeBlock:
			if enter {
				s.WriteByte(0x11)
				for i := 0; i < n.Lines().Len(); i++ {
					line := n.Lines().At(i)
					if i == 0 && len(line.Value(source)) == 0 {
						continue
					}
					s.WriteString("\x0314>\x03 ")
					s.Write(line.Value(source))
				}
				s.WriteByte(0x11)
			}
		case *ast.Link:
			if enter {
				fmt.Fprintf(&s, "\x0302%s[%s]\x03", n.Title, n.Destination)
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
				s.WriteByte(0x11)
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
					fmt.Fprintf(&s, "\x02\x0302@%s\x03\x02", n.GuildUser.Username)
				}
			}
		case *ast.String:
			if enter {
				s.Write(n.Value)
			}
		case *ast.Text:
			if enter {
				s.Write(n.Segment.Value(source))
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

func (c *Client) renderMessage(m *discord.Message, send func(string) error) error {
	if m.Type != discord.DefaultMessage {
		return nil
	}
	var s strings.Builder
	s.WriteString(c.renderContent([]byte(m.Content), m))
	for _, e := range m.Embeds {
		if e.Title != "" {
			fmt.Fprintf(&s, "\x02%s\x02", e.Title)
			if e.URL != "" {
				fmt.Fprintf(&s, " \x0302%s\x03", e.URL)
			}
			s.WriteString("\n")
		}

		if e.Description != "" {
			s.WriteString(c.renderContent([]byte(e.Description), m))
			s.WriteString("\n")
		}

		for _, f := range e.Fields {
			fmt.Fprintf(&s, "\x1D%s:\x1D ", f.Name)
			if !f.Inline {
				s.WriteString("\n")
			}
			s.WriteString(c.renderContent([]byte(e.Description), m))
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
			fmt.Fprintf(&s, " | \x0302%s\x03", a.Proxy)
		}
	}
	for _, s := range strings.Split(strings.Trim(s.String(), "\n"), "\n") {
		if err := send(s); err != nil {
			return err
		}
	}
	return nil
}
