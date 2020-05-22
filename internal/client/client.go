package client

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

type Client struct {
	conn           net.Conn
	irc            *irc.Conn
	session        *session.Session
	guild          *discord.Guild
	serverPrefix   *irc.Prefix
	clientPrefix   *irc.Prefix
	lastMessageID  discord.Snowflake // the ID of the last message this client sent
	caps           map[string]bool
	joinedChannels map[discord.Snowflake]bool
	IRCDebug       bool
	DiscordDebug   bool
}

func New(conn net.Conn) *Client {
	return &Client{
		conn:           conn,
		irc:            irc.NewConn(conn),
		serverPrefix:   &irc.Prefix{Name: conn.LocalAddr().String()},
		clientPrefix:   &irc.Prefix{Name: conn.RemoteAddr().String()},
		caps:           make(map[string]bool),
		joinedChannels: make(map[discord.Snowflake]bool),
	}
}

func (c *Client) Close() error {
	if c.session != nil {
		c.session.Unref()
	}
	return c.conn.Close()
}

func (c *Client) WriteMessage(m *irc.Message) error {
	if c.IRCDebug {
		log.Printf("->i %s", m)
	}
	return c.irc.WriteMessage(m)
}

func (c *Client) ReadMessage() (*irc.Message, error) {
	m, err := c.irc.ReadMessage()
	if c.IRCDebug && err == nil {
		log.Printf("<-i %s", m)
	}
	return m, err
}

var supportedCaps = []string{
	"echo-message",
	"server-time",
	"message-tags",
}
var supportedCapsString = strings.Join(supportedCaps, " ")
var supportedCapsSet = func() map[string]struct{} {
	set := make(map[string]struct{})
	for _, capability := range supportedCaps {
		set[capability] = struct{}{}
	}
	return set
}()

func (c *Client) Run() error {
	defer c.Close()

	log.Printf("connected: %v", c.clientPrefix.Name)
	defer log.Printf("disconnected: %v", c.clientPrefix.Name)

	passed, nicked, usered, blocked := false, false, false, false
	for !passed || !nicked || !usered || blocked {
		msg, err := c.ReadMessage()
		if err != nil {
			return err
		}
		switch msg.Command {
		case "NICK":
			nicked = true
		case "USER":
			usered = true
		case "CAP":
			if len(msg.Params) < 1 {
				return fmt.Errorf("invalid parameter count for CAP")
			}
			switch msg.Params[0] {
			case "LS":
				c.WriteMessage(&irc.Message{
					Prefix:  c.serverPrefix,
					Command: "CAP",
					Params:  []string{c.clientPrefix.Name, "LS", supportedCapsString},
				})
				blocked = true
			case "REQ":
				if len(msg.Params) != 2 {
					return fmt.Errorf("invalid parameter count for CAP REQ")
				}
				for _, capability := range strings.Split(msg.Params[1], " ") {
					if _, ok := supportedCapsSet[capability]; !ok {
						return fmt.Errorf("invalid capability requested: %s", capability)
					}
					c.caps[capability] = true
				}
				c.WriteMessage(&irc.Message{
					Prefix:  c.serverPrefix,
					Command: "CAP",
					Params:  []string{c.clientPrefix.Name, "ACK", msg.Params[1]},
				})
				blocked = true
			case "END":
				blocked = false
			}
		case "PASS":
			if len(msg.Params) != 1 {
				return fmt.Errorf("invalid parameter count for PASS")
			}
			args := strings.SplitN(msg.Params[0], ":", 2)
			session, err := session.Get(args[0], c.DiscordDebug)
			if err != nil {
				return err
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
				c.session.Gateway.GuildSubscribe(gateway.GuildSubscribeData{
					GuildID: guild.ID,
				})
				c.guild = guild
			}
			passed = true
		default:
			return fmt.Errorf("invalid command received in authentication stage: %v",
				msg.Command)
		}
	}

	me, err := c.session.Me()
	if err != nil {
		return err
	}

	c.clientPrefix = c.discordUserPrefix(me)

	if err := c.seedState(); err != nil {
		return err
	}

	if err := c.sendGreeting(); err != nil {
		return err
	}

	msgs := make(chan *irc.Message)
	errors := make(chan error)

	go func() {
		for {
			msg, err := c.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			msgs <- msg
		}
	}()

	events, cancel := c.session.ChanFor(func(e interface{}) bool { return true })
	defer cancel()

	for {
		select {
		case msg := <-msgs:
			if err := c.handleIRCMessage(msg); err != nil {
				return err
			}
		case event := <-events:
			if err := c.handleDiscordEvent(event); err != nil {
				return err
			}
		case err := <-errors:
			return err
		}
	}
}

func (c *Client) channelIsVisible(channel *discord.Channel) (bool, error) {
	me, err := c.session.Me()
	if err != nil {
		return false, err
	}

	if channel.Type != discord.GuildText {
		return false, nil
	}

	perms, err := c.session.Permissions(channel.ID, me.ID)
	if err != nil {
		return false, err
	}

	return perms.Has(discord.PermissionViewChannel), nil
}

func (c *Client) seedState() error {
	if c.guild != nil {
		channels, err := c.session.Channels(c.guild.ID)
		if err != nil {
			return err
		}
		for _, channel := range channels {
			if visible, err := c.channelIsVisible(&channel); err != nil {
				return err
			} else if visible {
				c.session.ChannelMap(c.guild.ID).Insert(channel.ID, channel.Name)
			}
		}
	}
	return nil
}

func (c *Client) sendGreeting() error {
	me, err := c.session.Me()
	if err != nil {
		return err
	}

	guildName := "Discord"
	guildID := me.ID
	if c.guild != nil {
		guildName = c.guild.Name
		guildID = c.guild.ID
	}

	if err := c.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: irc.RPL_WELCOME,
		Params: []string{c.clientPrefix.Name, fmt.Sprintf("Welcome to %s, %s",
			guildName, c.clientPrefix.Name)},
	}); err != nil {
		return err
	}

	if err := c.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: irc.RPL_YOURHOST,
		Params: []string{c.clientPrefix.Name,
			fmt.Sprintf("Your host is %s, running ircdiscord", c.serverPrefix.Name)},
	}); err != nil {
		return err
	}

	if err := c.WriteMessage(&irc.Message{
		Prefix:  c.serverPrefix,
		Command: irc.RPL_CREATED,
		Params: []string{c.clientPrefix.Name,
			fmt.Sprintf("This server was created %s", guildID.Time().String())},
	}); err != nil {
		return err
	}

	return nil
}
