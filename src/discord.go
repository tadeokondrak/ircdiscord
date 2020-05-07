package ircdiscord

import (
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/diamondburned/ningen/md"
	"github.com/yuin/goldmark/ast"
	"gopkg.in/sorcix/irc.v2"
)

func (c *Client) renderMessage(m *discord.Message, send func(string) error) error {
	source := []byte(m.Content)
	parsed := md.ParseWithMessage(source, c.session.Store, m, true)
	var s strings.Builder
	err := ast.Walk(parsed, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Document:
			// intentionally left blank
		case *ast.Blockquote:
			s.WriteString("---\n")
		case *ast.Paragraph:
			if !enter {
				s.WriteString("\n")
			}
		case *ast.FencedCodeBlock:
			if enter {
				s.WriteByte(0x11)
				for i := 0; i < n.Lines().Len(); i++ {
					line := n.Lines().At(i)
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
				// TODO
			}
		case *md.Emoji:
			if enter {
				fmt.Fprintf(&s, "\x02:%s:\x02", n.Name)
			}
		case *md.Mention:
			if enter {
				switch {
				case n.Channel != nil:
					fmt.Fprintf(&s, "\x0302#%s\x03", n.Channel.Name)
				case n.GuildUser != nil:
					fmt.Fprintf(&s, "\x0302@%s\x03", n.GuildUser.Username)
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
	})
	for _, s := range strings.Split(strings.Trim(s.String(), "\n"), "\n") {
		if err := send(s); err != nil {
			return err
		}
	}
	return err
}

func (c *Client) handleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.MessageCreateEvent:
		name, ok := c.subscribedChannels[e.ChannelID]
		if !ok || e.ID == c.lastMessageID {
			return nil
		}
		return c.renderMessage(&e.Message, func(s string) error {
			return c.irc.Encode(&irc.Message{
				Prefix: &irc.Prefix{
					User: ircClean(e.Author.Username),
					Name: ircClean(e.Author.Username),
					Host: e.Author.ID.String(),
				},
				Command: irc.PRIVMSG,
				Params:  []string{fmt.Sprintf("#%s", name), s},
			})
		})

	}
	return nil
}
