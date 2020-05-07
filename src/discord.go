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
	parsed := md.ParseWithMessage(source, c.session.Store, m, false)
	var s strings.Builder
	err := ast.Walk(parsed, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Document:
			// noop
		case *ast.Blockquote:
			s.WriteString("---\n")
		case *ast.Paragraph:
			if !enter {
				s.WriteString("\n")
			}
		case *ast.FencedCodeBlock:
			if enter {
				// Write the body
				for i := 0; i < n.Lines().Len(); i++ {
					line := n.Lines().At(i)
					s.WriteString("▏▕  " + string(line.Value(source)))
				}
			}
		case *ast.Link:
			if enter {
				s.WriteString(string(n.Title) + " (" + string(n.Destination) + ")")
			}
		case *ast.AutoLink:
			if enter {
				s.WriteString(string(n.URL(source)))
			}
		case *md.Inline:
			// n.Attr should be used, but since we're in plaintext mode, there is no
			// formatting.
		case *md.Emoji:
			if enter {
				s.WriteString(":" + string(n.Name) + ":")
			}
		case *md.Mention:
			if enter {
				switch {
				case n.Channel != nil:
					s.WriteString("#" + n.Channel.Name)
				case n.GuildUser != nil:
					s.WriteString("@" + n.GuildUser.Username)
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
