package main

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"gopkg.in/sorcix/irc.v2"
)

type Client struct {
	conn               net.Conn
	irc                *irc.Conn
	session            *Session
	guild              *discord.Guild
	subscribedChannels map[discord.Snowflake]string
	serverPrefix       irc.Prefix
	clientPrefix       irc.Prefix
	lastMessageID      discord.Snowflake
}

func NewClient(conn net.Conn) *Client {
	return &Client{
		conn:               conn,
		irc:                irc.NewConn(conn),
		subscribedChannels: make(map[discord.Snowflake]string),
	}
}

func (c *Client) Close() error {
	if c.session != nil {
		c.session.Unref()
	}
	return c.irc.Close()
}

func (c *Client) Run() error {
	defer c.Close()

	c.serverPrefix.Name = c.conn.LocalAddr().String()
	c.clientPrefix.Name = c.conn.RemoteAddr().String()

	addr := c.conn.RemoteAddr()
	log.Printf("connected: %v", addr)
	defer log.Printf("disconnected: %v", addr)

initial_loop:
	for {
		msg, err := c.irc.Decode()
		if err != nil {
			return fmt.Errorf("error decoding message: %v", err)
		}
		switch msg.Command {
		case irc.CAP, irc.NICK, irc.USER:
			// intentionally left blank
		case irc.PASS:
			if len(msg.Params) != 1 {
				return fmt.Errorf("invalid parameter count for PASS")
			}
			args := strings.SplitN(msg.Params[0], ":", 2)
			session, err := GetSession(args[0])
			if err != nil {
				return fmt.Errorf("failed to get session: %v", err)
			}
			c.session = session
			if len(args) > 1 {
				snowflake, err := discord.ParseSnowflake(args[1])
				if err != nil {
					return err
				}
				guild, err := c.session.Guild(snowflake)
				if err != nil {
					return err
				}
				c.guild = guild
			}
			break initial_loop
		default:
			return fmt.Errorf("invalid command received for auth stage: %v",
				msg.Command)
		}
	}

	me, err := c.session.Me()
	if err != nil {
		return fmt.Errorf("failed to get own user: %v", err)
	}

	c.clientPrefix.User = me.Username
	c.clientPrefix.Name = me.Username
	c.clientPrefix.Host = me.ID.String()

	c.irc.Encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.RPL_WELCOME,
		Params: []string{c.clientPrefix.Name, fmt.Sprintf("Welcome to IRCdiscord, %s#%s",
			me.Username, me.Discriminator)},
	})

	msgs, errors := c.DecodeChan()
	events, cancel := c.session.ChanFor(func(e interface{}) bool { return true })
	defer cancel()

	for {
		select {
		case msg := <-msgs:
			if err := c.HandleIRCMessage(msg); err != nil {
				return err
			}
		case event := <-events:
			if err := c.HandleDiscordEvent(event); err != nil {
				return err
			}
		case err := <-errors:
			return err
		}
	}
}

func (c *Client) DecodeChan() (<-chan *irc.Message, <-chan error) {
	msgs := make(chan *irc.Message)
	errors := make(chan error)
	go func() {
		for {
			msg, err := c.irc.Decode()
			if err != nil {
				errors <- fmt.Errorf("error decoding message: %v", err)
				close(msgs)
				close(errors)
				return
			}
			msgs <- msg
		}
	}()
	return msgs, errors
}

func (c *Client) HandleIRCMessage(msg *irc.Message) error {
	switch msg.Command {
	case irc.PING:
		return c.HandleIRCPing(msg)
	case irc.JOIN:
		return c.HandleIRCJoin(msg)
	case irc.PRIVMSG:
		return c.HandleIRCPrivmsg(msg)
	}
	return nil
}

func (c *Client) HandleIRCPing(msg *irc.Message) error {
	return c.irc.Encode(&irc.Message{
		Command: irc.PONG,
		Params:  msg.Params,
	})
}

func (c *Client) HandleIRCJoin(msg *irc.Message) error {
	if len(msg.Params) != 1 {
		return fmt.Errorf("invalid parameter count for JOIN")
	}
	for _, name := range strings.Split(msg.Params[0], ",") {
		if !strings.HasPrefix(name, "#") {
			return fmt.Errorf("invalid channel name")
		}
		name = name[1:]
		if c.guild == nil {
			return fmt.Errorf("JOIN for non-guilds is currently unimplemented")
		}
		channels, err := c.session.Channels(c.guild.ID)
		if err != nil {
			return err
		}
		var found *discord.Channel
		for _, channel := range channels {
			if channel.Name == name {
				found = &channel
				break
			}
		}
		if found == nil {
			return fmt.Errorf("unknown channel %s", name)
		}
		c.subscribedChannels[found.ID] = name
		err = c.irc.Encode(&irc.Message{
			Prefix:  &c.clientPrefix,
			Command: irc.JOIN,
			Params:  []string{fmt.Sprintf("#%s", name)},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) HandleIRCPrivmsg(msg *irc.Message) error {
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
