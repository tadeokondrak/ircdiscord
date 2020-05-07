package ircdiscord

import (
	"fmt"

	"github.com/diamondburned/arikawa/gateway"
	"gopkg.in/sorcix/irc.v2"
)

func (c *Client) handleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.MessageCreateEvent:
		return c.handleDiscordMessageCreate(e)
	default:
		return nil
	}
}

func (c *Client) handleDiscordMessageCreate(e *gateway.MessageCreateEvent) error {
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
