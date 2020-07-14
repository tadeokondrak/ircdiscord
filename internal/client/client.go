package client

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

type Client struct {
	*irc.Conn
	net            net.Conn
	session        *session.Session
	guild          discord.Snowflake
	serverPrefix   *irc.Prefix
	clientPrefix   *irc.Prefix
	joinedChannels map[string]bool
	// the ID of the last message sent by this client
	// this is used to prevent duplicate messages for clients who don't support
	// ircv3 echo-message
	lastMessageID discord.Snowflake
	// ircv3 capabilities
	caps         map[string]bool
	IRCDebug     bool
	DiscordDebug bool
}

func New(conn net.Conn) *Client {
	c := &Client{
		Conn:           irc.NewConn(conn),
		net:            conn,
		serverPrefix:   &irc.Prefix{Name: conn.LocalAddr().String()},
		clientPrefix:   &irc.Prefix{Name: conn.RemoteAddr().String()},
		joinedChannels: make(map[string]bool),
		caps:           make(map[string]bool),
	}
	c.Conn.Reader.DebugCallback = func(line string) {
		if c.IRCDebug {
			log.Printf("<-i %s", line)
		}
	}
	c.Conn.Writer.DebugCallback = func(line string) {
		if c.IRCDebug {
			log.Printf("->i %s", line)
		}
	}
	return c
}

func (c *Client) Close() error {
	if c.session != nil {
		c.session.Unref()
	}
	return c.net.Close()
}

func (c *Client) HasCapability(capability string) bool {
	return c.caps[capability]
}

func (c *Client) ClientPrefix() *irc.Prefix {
	return c.clientPrefix
}

func (c *Client) ServerPrefix() *irc.Prefix {
	return c.serverPrefix
}

var supportedCaps = []string{
	"echo-message",
	"server-time",
	"message-tags",
}

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
				if err := replies.CAP_LS(c, supportedCaps); err != nil {
					return err
				}
				blocked = true
			case "REQ":
				if len(msg.Params) != 2 {
					return fmt.Errorf("invalid parameter count for CAP REQ")
				}
				requested := strings.Split(msg.Params[1], " ")
				for _, capability := range requested {
					supported := false
					for _, supportedCap := range supportedCaps {
						if supportedCap == capability {
							supported = true
							break
						}
					}
					if !supported {
						return fmt.Errorf("invalid capability requested: %s", capability)
					}
					c.caps[capability] = true
				}
				if err := replies.CAP_ACK(c, requested); err != nil {
					return err
				}
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
				c.guild = guild.ID
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

	if err := c.seedState(); err != nil {
		return err
	}

	c.clientPrefix = c.discordUserPrefix(me)

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

func (c *Client) sendGreeting() error {
	me, err := c.session.Me()
	if err != nil {
		return err
	}

	guildName := "Discord"
	guildID := me.ID
	if c.guild.Valid() {
		guild, err := c.session.Guild(c.guild)
		if err != nil {
			return err
		}
		guildName = guild.Name
		guildID = c.guild
	}

	if err := replies.RPL_WELCOME(c, guildName); err != nil {
		return err
	}

	if err := replies.RPL_YOURHOST(c); err != nil {
		return err
	}

	if err := replies.RPL_CREATED(c, guildID.Time()); err != nil {
		return err
	}

	if err := replies.ERR_NOMOTD(c); err != nil {
		return err
	}

	return nil
}

func (c *Client) seedState() error {
	if c.guild.Valid() {
		return nil
	}

	channels, err := c.session.PrivateChannels()
	if err != nil {
		return err
	}

	for _, channel := range channels {
		if channel.Type != discord.DirectMessage {
			continue
		}
		recip := channel.DMRecipients[0]
		c.session.UserName(recip.ID, recip.Username)
	}

	return nil
}
