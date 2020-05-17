package ircdiscord

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/tadeokondrak/ircdiscord/render"
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

	err = c.WriteMessage(&irc.Message{
		Prefix:  &c.clientPrefix,
		Command: "JOIN",
		Params:  []string{fmt.Sprintf("#%s", name)},
	})
	if err != nil {
		return err
	}

	if found.Topic != "" {
		err = c.WriteMessage(&irc.Message{
			Prefix:  &c.serverPrefix,
			Command: irc.RPL_TOPIC,
			Params: []string{
				c.clientPrefix.Name,
				fmt.Sprintf("#%s", name),
				render.Content(c.session, []byte(found.Topic), nil),
			},
		})
	}

	if err := c.WriteMessage(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: "329", // RPL_CREATIONTIME
		Params: []string{
			c.clientPrefix.Name,
			fmt.Sprintf("#%s", name),
			fmt.Sprint(found.ID.Time().Unix()),
		},
	}); err != nil {
		return err
	}

	backlog, err := c.session.Messages(found.ID)
	for i := len(backlog) - 1; i >= 0; i-- {
		if err := c.sendDiscordMessage(&backlog[i]); err != nil {
			return err
		}
	}

	return err
}

func (c *Client) sendMessage(channel, content string) error {
	var found *discord.Snowflake
	for id, name := range c.subscribedChannels {
		if name == channel {
			found = &id
			break
		}
	}
	if found == nil {
		return fmt.Errorf("unknown channel %s", channel)
	}
	msg, err := c.session.SendMessage(*found, content, nil)
	if err != nil {
		return err
	}
	c.lastMessageID = msg.ID
	return nil
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
	return c.WriteMessage(&irc.Message{
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

var actionRegex = regexp.MustCompile(`^\x01ACTION (.*)\x01$`)

func (c *Client) handleIRCPrivmsg(msg *irc.Message) error {
	if len(msg.Params) < 2 {
		return fmt.Errorf("not enough parameters for PRIVMSG")
	}
	if !strings.HasPrefix(msg.Params[0], "#") {
		return fmt.Errorf("invalid channel name")
	}
	text := msg.Params[1]
	text = actionRegex.ReplaceAllString(text, "*$1*")
	return c.sendMessage(msg.Params[0][1:], text)
}
