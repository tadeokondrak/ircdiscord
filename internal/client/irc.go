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
	if !c.guild.Valid() {
		return errors.New("JOIN for non-guilds is currently unsupported")
	}

	name, err := c.session.ChannelName(c.guild, channel.ID)
	if err != nil {
		return err
	}

	err = c.irc.WriteMessage(&irc.Message{
		Prefix:  c.clientPrefix,
		Command: "JOIN",
		Params:  []string{name},
	})
	if err != nil {
		return err
	}

	if channel.Topic != "" {
		err = c.irc.WriteMessage(&irc.Message{
			Prefix:  c.serverPrefix,
			Command: irc.RPL_TOPIC,
			Params: []string{
				c.clientPrefix.Name,
				name,
				strings.ReplaceAll(render.Content(c.session, []byte(channel.Topic), nil), "\n", " "),
			},
		})
	}

	if err := c.irc.WriteMessage(&irc.Message{
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

func (c *Client) handleIRCMessage(msg *irc.Message) error {
	switch msg.Command {
	case "PING":
		return c.handleIRCPing(msg)
	case "JOIN":
		return c.handleIRCJoin(msg)
	case "PRIVMSG":
		return c.handleIRCPrivmsg(msg)
	case "LIST":
		return c.handleIRCList(msg)
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
		channelID := c.session.ChannelFromName(c.guild, name[1:])
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
	if strings.HasPrefix(text, "s/") {
		return c.handleIRCRegexEdit(msg)
	}
	text = actionRegex.ReplaceAllString(text, "*$1*")
	text = c.replaceIRCMentions(text)
	channel := c.session.ChannelFromName(c.guild, strings.TrimPrefix(msg.Params[0], "#"))
	if !channel.Valid() {
		return fmt.Errorf("Invalid channel")
	}
	dmsg, err := c.session.SendMessage(channel, msg.Params[1], nil)
	if err != nil {
		return err
	}
	c.lastMessageID = dmsg.ID

	return nil
}

var editRegex = regexp.MustCompile(`^s/((?:\\/|[^/])*)/((?:\\/|[^/])*)(?:/(g?))?$`)

func (c *Client) handleIRCRegexEdit(msg *irc.Message) error {
	bail := func(format string, args ...interface{}) error {
		return c.irc.WriteMessage(&irc.Message{
			Prefix:  c.serverPrefix,
			Command: "NOTICE",
			Params:  []string{msg.Params[0], fmt.Sprintf(format, args...)}},
		)
	}

	matches := editRegex.FindStringSubmatch(msg.Params[1])
	if matches == nil {
		return bail("invalid replacement")
	}

	regex, err := regexp.Compile(matches[1])
	if err != nil {
		return bail("failed to compile regex: %v", err)
	}

	channel := c.session.ChannelFromName(c.guild, strings.TrimPrefix(msg.Params[0], "#"))
	if !channel.Valid() {
		return fmt.Errorf("failed to find channel #%s", msg.Params[0])
	}

	backlog, err := c.session.Messages(channel)
	if err != nil {
		return err
	}

	me, err := c.session.Me()
	if err != nil {
		return err
	}

	var snowflake discord.Snowflake
	for _, msg := range backlog {
		if msg.Author.ID == me.ID {
			snowflake = msg.ID
			break
		}
	}

	if !snowflake.Valid() {
		return bail("failed to find your message")
	}

	message, err := c.session.Message(channel, snowflake)
	if err != nil {
		return err
	}

	beforeEdit := message.Content

	var result string

	if matches[3] == "g" {
		fmt.Printf("%s,%s,%s", regex, beforeEdit, matches)
		result = regex.ReplaceAllString(beforeEdit, matches[2])
	} else {
		match := regex.FindStringSubmatchIndex(beforeEdit)
		if match == nil {
			return bail("no matches")
		}
		dst := []byte{}
		replaced := regex.ExpandString(dst, matches[2], beforeEdit, match)
		result = beforeEdit[:match[0]] + string(replaced) + beforeEdit[match[1]:]
	}

	_, err = c.session.EditMessage(message.ChannelID, message.ID, string(result), nil, false)
	return err
}

var pingRegex = regexp.MustCompile(`@[^ ]*`)

func (c *Client) replaceIRCMentions(s string) string {
	return pingRegex.ReplaceAllStringFunc(s, func(match string) string {
		if match == "@" {
			return match
		}
		id := c.session.UserFromName(match[1:])
		if !id.Valid() {
			return match
		}
		return fmt.Sprintf("<@%d>", id)
	})
}

func (c *Client) handleIRCList(msg *irc.Message) error {
	if !c.guild.Valid() {
		// TODO: support /LIST for non-guilds
		return nil
	}
	guild, err := c.session.Guild(c.guild)
	if err != nil {
		return err
	}

	if err := c.irc.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: irc.RPL_LISTSTART,
		Params:  []string{c.clientPrefix.Name, fmt.Sprintf("Channel list for %s", guild.Name)},
	}); err != nil {
		return err
	}
	channels, err := c.session.Channels(c.guild)
	if err != nil {
		return err
	}
	for _, channel := range channels {
		if visible, err := c.channelIsVisible(&channel); err != nil {
			return err
		} else if !visible {
			continue
		}
		name, err := c.session.ChannelName(c.guild, channel.ID)
		if err != nil {
			return err
		}
		if err := c.irc.WriteMessage(&irc.Message{
			Prefix:  c.serverPrefix,
			Command: irc.RPL_LIST,
			Params: []string{c.clientPrefix.Name, name, fmt.Sprint(channel.Position),
				strings.ReplaceAll(render.Content(c.session, []byte(channel.Topic), nil), "\n", " ")},
		}); err != nil {
			return err
		}
	}
	if err := c.irc.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: irc.RPL_LISTEND,
		Params:  []string{c.clientPrefix.Name, "End of /LIST"},
	}); err != nil {
		return err
	}
	return nil
}
