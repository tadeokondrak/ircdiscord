package client

import (
	"fmt"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/render"
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
	tags := make(irc.Tags)
	if c.caps["server-time"] {
		tags["time"] = irc.TagValue(m.ID.Time().Format("2006-01-02T15:04:05.000Z"))
	}
	return render.Message(c.session, m, func(s string) error {
		return c.WriteMessage(&irc.Message{
			Tags:    tags,
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
	if m.ID == c.lastMessageID && !c.caps["echo-message"] {
		return nil
	}
	return c.sendDiscordMessage(&m)
}
