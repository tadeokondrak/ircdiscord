package client

import (
	"log"
	"net"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/tadeokondrak/ircdiscord/internal/ilayer"
	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

type Client struct {
	netconn       net.Conn
	ircconn       *irc.Conn
	ilayer        *ilayer.Client
	session       *session.Session  // nil pre-login
	guild         discord.Snowflake // invalid for DM server and pre-login
	lastMessageID discord.Snowflake // used to prevent duplicate messages
	capabilities  map[string]bool   // ircv3 capabilities
	discordDebug  bool              // whether to log Discord interaction
	errors        chan error        // send errors here from goroutines
}

func New(conn net.Conn, ircDebug, discordDebug bool) *Client {
	ircconn := irc.NewConn(conn)
	client := ilayer.NewClient(ircconn,
		conn.LocalAddr().String(), conn.RemoteAddr().String())

	c := &Client{
		netconn:      conn,
		ircconn:      ircconn,
		ilayer:       client,
		capabilities: make(map[string]bool),
		discordDebug: discordDebug,
		errors:       make(chan error),
	}

	c.ilayer.Server = c

	if ircDebug {
		c.ircconn.Reader.DebugCallback = func(line string) {
			log.Printf("<-i %s", line)
		}

		c.ircconn.Writer.DebugCallback = func(line string) {
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
	log.Printf("connected: %v", c.ilayer.ClientPrefix().Name)
	defer log.Printf("disconnected: %v", c.ilayer.ClientPrefix().Name)

	msgs := make(chan *irc.Message)

	go func() {
		for {
			msg, err := c.ilayer.ReadMessage()
			if err != nil {
				c.errors <- err
				return
			}
			msgs <- msg
		}
	}()

	for !c.ilayer.IsRegistered() {
		select {
		case msg := <-msgs:
			if err := c.ilayer.HandleMessage(msg); err != nil {
				return err
			}
		case err := <-c.errors:
			return err
		}
	}

	events, cancel := c.session.ChanFor(
		func(interface{}) bool { return true })
	defer cancel()

	sessionEvents, sessionCancel := c.session.SessionHandler.ChanFor(
		func(interface{}) bool { return true })
	defer sessionCancel()

	for {
		select {
		case msg := <-msgs:
			if err := c.ilayer.HandleMessage(msg); err != nil {
				return err
			}
		case event := <-events:
			if err := c.handleDiscordEvent(event); err != nil {
				return err
			}
		case event := <-sessionEvents:
			if err := c.handleSessionEvent(event); err != nil {
				return err
			}
		case err := <-c.errors:
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
			c.session.UserName(c.guild, recip.ID)
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

func (c *Client) handleSessionEvent(e gateway.Event) error {
	switch e := e.(type) {
	case *session.UserNameChange:
		if e.GuildID == c.guild {
			prefix := &irc.Prefix{
				User: e.Old,
				Name: e.Old,
				Host: e.ID.String(),
			}
			replies.NICK(c.ilayer, prefix, e.New)
		}
	case *session.ChannelNameChange:
	}
	return nil
}
