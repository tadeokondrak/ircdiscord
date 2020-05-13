package ircdiscord

import (
	"fmt"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"gopkg.in/irc.v3"
)

func discordUserPrefix(u *discord.User) *irc.Prefix {
	return &irc.Prefix{
		User: ircUsername(u.Username),
		Name: ircUsername(u.Username),
		Host: u.ID.String(),
	}
}

func (c *Client) sendDiscordMessage(m *discord.Message) error {
	channel, ok := c.subscribedChannels[m.ChannelID]
	if !ok {
		return nil
	}
	return c.renderMessage(m, func(s string) error {
		return c.WriteMessage(&irc.Message{
			Prefix:  discordUserPrefix(&m.Author),
			Command: "PRIVMSG",
			Params:  []string{fmt.Sprintf("#%s", channel), s},
		})
	})
}

func (c *Client) handleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.MessageCreateEvent:
		return c.handleDiscordMessage(e.Message)
	case *gateway.MessageUpdateEvent:
		return c.handleDiscordMessage(e.Message)
	default:
		return nil
	}
}

func (c *Client) handleDiscordMessage(m discord.Message) error {
	if m.ID == c.lastMessageID {
		return nil
	}
	return c.sendDiscordMessage(&m)
}
