package ircdiscord

import (
	"fmt"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"gopkg.in/irc.v3"
)

func (c *Client) sendDiscordMessage(m *discord.Message) error {
	channel, ok := c.subscribedChannels[m.ChannelID]
	if !ok {
		return nil
	}
	return c.renderMessage(m, func(s string) error {
		return c.irc.WriteMessage(&irc.Message{
			Prefix: &irc.Prefix{
				User: ircUsername(m.Author.Username),
				Name: ircUsername(m.Author.Username),
				Host: m.Author.ID.String(),
			},
			Command: "PRIVMSG",
			Params:  []string{fmt.Sprintf("#%s", channel), s},
		})
	})
}

func (c *Client) handleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.MessageCreateEvent:
		return c.handleDiscordMessageCreate(e)
	default:
		return nil
	}
}

func (c *Client) handleDiscordMessageCreate(e *gateway.MessageCreateEvent) error {
	if e.ID == c.lastMessageID {
		return nil
	}
	return c.sendDiscordMessage(&e.Message)
}
