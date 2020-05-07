package ircdiscord

import (
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"gopkg.in/irc.v3"
)

func (c *Client) joinChannel(name string) error {
	if c.guild == nil {
		return fmt.Errorf("JOIN for non-guilds is currently unimplemented")
	}

	channels, err := c.session.Channels(c.guild.ID)
	if err != nil {
		return err
	}

	var found *discord.Channel
	for _, channel := range channels {
		if channel.Name == name && channel.Type == discord.GuildText {
			found = &channel
			break
		}
	}

	if found == nil {
		return fmt.Errorf("unknown channel %s", name)
	}

	c.subscribedChannels[found.ID] = name

	err = c.irc.WriteMessage(&irc.Message{
		Prefix:  &c.clientPrefix,
		Command: "JOIN",
		Params:  []string{fmt.Sprintf("#%s", name)},
	})
	if err != nil {
		return err
	}

	if found.Topic != "" {
		err = c.irc.WriteMessage(&irc.Message{
			Prefix:  &c.serverPrefix,
			Command: irc.RPL_TOPIC,
			Params: []string{
				c.clientPrefix.Name,
				fmt.Sprintf("#%s", name),
				c.renderContent([]byte(found.Topic), nil),
			},
		})
	}

	err = c.irc.WriteMessage(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: "329", // RPL_CREATIONTIME
		Params: []string{
			c.clientPrefix.Name,
			fmt.Sprintf("#%s", name),
			fmt.Sprint(found.ID.Time().Unix()),
		},
	})

	return err
}

func (c *Client) handleIRCMessage(msg *irc.Message) error {
	switch msg.Command {
	case "PING":
		return c.handleIRCPing(msg)
	case "JOIN":
		return c.handleIRCJoin(msg)
	case "PRIVMSG":
		return c.handleIRCPrivmsg(msg)
	default:
		return nil
	}
}

func (c *Client) handleIRCPing(msg *irc.Message) error {
	return c.irc.WriteMessage(&irc.Message{
		Command: "PONG",
		Params:  msg.Params,
	})
}

func (c *Client) handleIRCJoin(msg *irc.Message) error {
	if len(msg.Params) != 1 {
		return fmt.Errorf("invalid parameter count for JOIN")
	}
	for _, name := range strings.Split(msg.Params[0], ",") {
		if !strings.HasPrefix(name, "#") {
			return fmt.Errorf("invalid channel name")
		}
		if err := c.joinChannel(name[1:]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) handleIRCPrivmsg(msg *irc.Message) error {
	if len(msg.Params) < 2 {
		return fmt.Errorf("not enough parameters for PRIVMSG")
	}
	if !strings.HasPrefix(msg.Params[0], "#") {
		return fmt.Errorf("invalid channel name")
	}
	name := msg.Params[0][1:]
	var id discord.Snowflake
	var channel string
	var found bool
	for id, channel = range c.subscribedChannels {
		if channel == name {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown channel %s", name)
	}
	dmsg, err := c.session.SendMessage(id, msg.Params[1], nil)
	if err != nil {
		return err
	}
	c.lastMessageID = dmsg.ID
	return nil
}
