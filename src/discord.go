package ircdiscord

import (
	"fmt"

	"github.com/diamondburned/arikawa/gateway"
	"gopkg.in/sorcix/irc.v2"
)

func (c *Client) HandleDiscordEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *gateway.MessageCreateEvent:
		name, ok := c.subscribedChannels[e.ChannelID]
		if !ok || e.ID == c.lastMessageID {
			return nil
		}
		return c.irc.Encode(&irc.Message{
			Prefix: &irc.Prefix{
				User: e.Author.Username,
				Name: e.Author.Username,
				Host: e.Author.ID.String(),
			},
			Command: irc.PRIVMSG,
			Params:  []string{fmt.Sprintf("#%s", name), e.Content},
		})

	}
	return nil
}
