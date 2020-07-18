package client

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/tadeokondrak/ircdiscord/internal/ilayer"
	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

type SessionFunc func(token string, debug bool) (*session.Session, error)

type Client struct {
	sessionFunc   SessionFunc
	netconn       net.Conn
	ircconn       *irc.Conn
	ilayer        *ilayer.Client
	session       *session.Session  // nil pre-login
	guild         discord.Snowflake // invalid for DM server and pre-login
	lastMessageID discord.Snowflake // used to prevent duplicate messages
	capabilities  map[string]bool   // ircv3 capabilities
	debug         bool
	discordDebug  bool       // whether to log Discord interaction
	errors        chan error // send errors here from goroutines
	cancels       []func()
}

func New(conn net.Conn, sessionFunc SessionFunc,
	debug, ircDebug, discordDebug bool) *Client {
	log.Printf("creating client %v", conn.RemoteAddr())

	ircconn := irc.NewConn(conn)
	client := ilayer.NewClient(ircconn,
		conn.LocalAddr().String(), conn.RemoteAddr().String())

	c := &Client{
		sessionFunc:  sessionFunc,
		netconn:      conn,
		ircconn:      ircconn,
		ilayer:       client,
		capabilities: make(map[string]bool),
		debug:        debug,
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
	if c.debug {
		log.Printf("closing client %v", c.netconn.RemoteAddr())
	}

	for _, cancel := range c.cancels {
		cancel()
	}

	if c.session != nil {
		c.session.Unref()
	}

	return c.netconn.Close()
}

func (c *Client) isGuild() bool {
	return c.guild.Valid()
}

const errClosingStr = "use of closed network connection"

func (c *Client) ircReadLoop(msgs chan<- *irc.Message) {
	for {
		msg, err := c.ilayer.ReadMessage()
		if err != nil {
			if err == io.EOF ||
				strings.Contains(err.Error(), errClosingStr) {
				err = nil
			}
			c.errors <- err
			return
		}
		msgs <- msg
	}
}

func (c *Client) Run() error {
	msgs := make(chan *irc.Message)
	go c.ircReadLoop(msgs)

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

	listCancel := c.session.SubscribeUserList(c.guild,
		func(e *session.UserNameChange) {
			c.handleUsernameChange(e, "")
		})
	defer listCancel()

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

// This function is called from multiple goroutines.
func (c *Client) handleUsernameChange(e *session.UserNameChange,
	channel string) {
	if e.GuildID == c.guild && c.isGuild() {
		if e.Old != "" {
			if channel == "" {
				prefix := &irc.Prefix{
					User: e.Old,
					Name: e.Old,
					Host: e.ID.String(),
				}
				replies.NICK(c.ilayer, prefix, e.New)
			}
		} else {
			prefix := &irc.Prefix{
				User: e.New,
				Name: e.New,
				Host: e.ID.String(),
			}
			if channel != "" {
				replies.JOIN(c.ilayer, prefix, channel)
			}
		}
	}
}
