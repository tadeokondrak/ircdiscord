package client

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/tadeokondrak/ircdiscord/internal/render"
	"gopkg.in/irc.v3"
)

func (c *Client) sendJoin(channel *discord.Channel) error {
	if c.guild == nil {
		return errors.New("JOIN for non-guilds is currently unsupported")
	}

	name, err := c.session.ChannelName(c.guild.ID, channel.ID)
	if err != nil {
		return err
	}

	err = c.WriteMessage(&irc.Message{
		Prefix:  c.clientPrefix,
		Command: "JOIN",
		Params:  []string{name},
	})
	if err != nil {
		return err
	}

	if channel.Topic != "" {
		err = c.WriteMessage(&irc.Message{
			Prefix:  c.serverPrefix,
			Command: irc.RPL_TOPIC,
			Params: []string{
				c.clientPrefix.Name,
				name,
				render.Content(c.session, []byte(channel.Topic), nil),
			},
		})
	}

	if err := c.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: "329", // RPL_CREATIONTIME
		Params: []string{
			c.clientPrefix.Name,
			fmt.Sprintf("%s", name),
			fmt.Sprint(channel.ID.Time().Unix()),
		},
	}); err != nil {
		return err
	}

	backlog, err := c.session.Messages(channel.ID)
	for i := len(backlog) - 1; i >= 0; i-- {
		if err := c.sendDiscordMessage(&backlog[i]); err != nil {
			return err
		}
	}

	return err
}

func (c *Client) sendMessage(channelName, content string) error {
	channel := c.session.ChannelFromName(c.guild.ID, channelName)
	msg, err := c.session.SendMessage(channel, content, nil)
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
		channelID := c.session.ChannelFromName(c.guild.ID, name[1:])
		channel, err := c.session.Channel(channelID)
		if err != nil {
			return err
		}
		if err := c.sendJoin(channel); err != nil {
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
