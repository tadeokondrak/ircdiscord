package client

import (
	"log"
	"net"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/tadeokondrak/ircdiscord/internal/server"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

type Client struct {
	netconn       net.Conn
	ircconn       *irc.Conn
	client        *server.Client
	session       *session.Session  // nil pre-login
	guild         discord.Snowflake // invalid for DM server and pre-login
	lastMessageID discord.Snowflake // used to prevent duplicate messages
	capabilities  map[string]bool   // ircv3 capabilities
	IRCDebug      bool              // whether to log IRC interaction
	DiscordDebug  bool              // whether to log Discord interaction
}

func New(conn net.Conn) *Client {
	ircconn := irc.NewConn(conn)
	client := server.NewClient(ircconn,
		conn.LocalAddr().String(), conn.RemoteAddr().String())

	c := &Client{
		netconn:      conn,
		ircconn:      ircconn,
		client:       client,
		capabilities: make(map[string]bool),
	}

	c.client.Server = c

	c.ircconn.Reader.DebugCallback = func(line string) {
		if c.IRCDebug {
			log.Printf("<-i %s", line)
		}
	}

	c.ircconn.Writer.DebugCallback = func(line string) {
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

	return c.netconn.Close()
}

func (c *Client) isGuild() bool {
	return c.guild.Valid()
}

func (c *Client) Run() error {
	defer c.Close()

	log.Printf("connected: %v", c.client.ClientPrefix().Name)
	defer log.Printf("disconnected: %v", c.client.ClientPrefix().Name)

	msgs := make(chan *irc.Message)
	errors := make(chan error)

	go func() {
		for {
			msg, err := c.client.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			msgs <- msg
		}
	}()

	for !c.client.IsRegistered() {
		select {
		case msg := <-msgs:
			if err := c.client.HandleMessage(msg); err != nil {
				return err
			}
		case err := <-errors:
			return err
		}
	}

	trueFunc := func(e interface{}) bool { return true }
	events, cancel := c.session.ChanFor(trueFunc)
	defer cancel()

	if err := c.seedState(); err != nil {
		return err
	}

	for {
		select {
		case msg := <-msgs:
			if err := c.client.HandleMessage(msg); err != nil {
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
	if c.isGuild() {
		channels, err := c.session.Channels(c.guild)
		if err != nil {
			return err
		}

		for _, channel := range channels {
			if channel.Type != discord.GuildText {
				continue
			}
			_, err := c.session.ChannelName(c.guild, channel.ID)
			if err != nil {
				return err
			}
		}
	} else {
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
	}

	return nil
}

func (c *Client) NetworkName() (string, error) {
	guildName := "Discord"

	if c.isGuild() {
		guild, err := c.session.Guild(c.guild)
		if err != nil {
			return "", err
		}
		guildName = guild.Name
	}

	return guildName, nil
}

func (c *Client) ServerName() (string, error) {
	return "ircdiscord", nil
}

func (c *Client) ServerVersion() (string, error) {
	return "git", nil
}

func (c *Client) ServerCreated() (time.Time, error) {
	me, err := c.session.Me()
	if err != nil {
		return time.Time{}, err
	}

	guildID := me.ID
	if c.isGuild() {
		guildID = c.guild
	}

	return guildID.Time(), nil
}

func (c *Client) MOTD() ([]string, error) {
	return []string{}, nil
}
