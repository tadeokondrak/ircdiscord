package client

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/tadeokondrak/ircdiscord/internal/ilayer"
	"github.com/tadeokondrak/ircdiscord/internal/session"
	"gopkg.in/irc.v3"
)

const errClosingStr = "use of closed network connection"

type SessionFunc func(token string, debug bool) (*session.Session, error)

type Client struct {
	sessionFunc  SessionFunc
	netconn      net.Conn
	ircconn      *irc.Conn
	ilayer       *ilayer.Client
	session      *session.Session // nil pre-login
	guild        session.Guild    // invalid for DM server and pre-login
	debug        bool
	ircDebug     bool
	discordDebug bool
	errors       chan error // send errors here from goroutines
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
		debug:        debug,
		ircDebug:     ircDebug,
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
	if c.ircDebug {
		log.Printf("closing client %v", c.netconn.RemoteAddr())
	}

	if c.session != nil {
		c.session.Unref()
	}

	return c.netconn.Close()
}

func (c *Client) ircReadLoop(msgs chan<- *irc.Message) {
	for {
		msg, err := c.ilayer.ReadMessage()
		if err != nil {
			if errors.Is(err, irc.ErrZeroLengthMessage) {
				continue
			}
			if strings.Contains(err.Error(), errClosingStr) {
				err = io.EOF
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

	events := make(chan interface{})
	c.session.Register(events, c.guild)
	defer c.session.Unregister(events)

	for {
		select {
		case msg := <-msgs:
			if err := c.ilayer.HandleMessage(msg); err != nil {
				return err
			}
		case event := <-events:
			if err := c.handleSessionEvent(event); err != nil {
				return err
			}
		case err := <-c.errors:
			return err
		}
	}
}

func (c *Client) handleSessionEvent(ev interface{}) error {
	switch ev := ev.(type) {
	case *session.MessageEvent:
		return c.ilayer.Message(c.msgEventToMessage(ev))
	case *session.TypingEvent:
		prefix := &irc.Prefix{
			Name: ev.User.Nickname,
			User: ev.User.Username,
			Host: ev.User.ID,
		}
		channelName := fmt.Sprintf("#%s", ev.Channel)
		return c.ilayer.Typing(ev.Date, prefix, channelName)
	case *session.ChannelHistoryEvent:
		// TODO
		return nil
	case *session.UserUpdateEvent:
		return nil
	default:
		return nil
	}
}

func (c *Client) msgEventToMessage(ev *session.MessageEvent) *ilayer.Message {
	prefix := &irc.Prefix{
		Name: ev.User.Nickname,
		User: ev.User.Username,
		Host: ev.User.ID,
	}
	channelName := fmt.Sprintf("#%s", ev.Channel)
	return &ilayer.Message{
		Channel: channelName,
		Content: ev.Content,
		ID:      ev.ID,
		Author:  prefix,
		Date:    ev.Date,
	}
}

func (c *Client) NetworkName() (string, error) {
	return c.session.GuildName(c.guild)
}

func (c *Client) ServerName() (string, error) {
	return "ircdiscord", nil
}

func (c *Client) ServerVersion() (string, error) {
	return "git", nil
}

func (c *Client) ServerCreated() (time.Time, error) {
	return c.session.GuildDate(c.guild)
}

func (c *Client) MOTD() ([]string, error) {
	var motd []string
	return motd, nil
}

func (c *Client) HandleRegister() error {
	password := c.ilayer.Password()
	if password == "" {
		password = c.ilayer.SASLPassword()
	}
	if password == "" {
		return fmt.Errorf("no password provided")
	}

	args := strings.SplitN(password, ":", 2)
	sess, err := c.sessionFunc(args[0], c.debug)
	if err != nil {
		return err
	}

	c.session = sess

	if len(args) > 1 {
		i, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}

		guildID := session.Guild(i)
		if err := c.session.ValidateGuild(guildID); err != nil {
			return err
		}

		c.guild = guildID
	}

	if c.ilayer.HasCapability("message-tags") && c.guild != 0 {
		c.session.TypingSubscribe(c.guild)
	}

	c.ilayer.SetClientPrefix(&irc.Prefix{
		Name: c.session.NickName(c.guild),
		User: c.session.UserName(),
		Host: fmt.Sprint(c.session.UserID()),
	})

	return nil
}

func (c *Client) HandleNickname(nickname string) (string, error) {
	if !c.ilayer.IsRegistered() {
		return nickname, nil
	}

	panic("TODO")
}

func (c *Client) HandleUsername(username string) (string, error) {
	return username, nil
}

func (c *Client) HandleRealname(realname string) (string, error) {
	return realname, nil
}

func (c *Client) HandlePassword(password string) (string, error) {
	return password, nil
}

func (c *Client) HandlePing(nonce string) (string, error) {
	return nonce, nil
}

func (c *Client) HandleJoin(channel string) (string, error) {
	err := c.session.Channel(c.guild, channel)
	return channel, err
}

func (c *Client) HandleTopic(channel string) (string, error) {
	return c.session.ChannelTopic(c.guild, channel)
}

func (c *Client) HandleCreationTime(channel string) (time.Time, error) {
	return time.Time{}, nil
}

func (c *Client) HandleNames(channel string) ([]string, error) {
	return nil, nil
}

func (c *Client) HandleMessage(channel, content string) error {
	return c.session.SendMessage(c.guild, channel, content)
}

func (c *Client) HandleList() ([]ilayer.ListEntry, error) {
	entries := []ilayer.ListEntry{}
	return entries, nil
}

func (c *Client) HandleWhois(username string) (ilayer.WhoisReply, error) {
	panic("todo")
}

func (c *Client) HandleChatHistoryBefore(channel string, t time.Time,
	limit int) ([]ilayer.Message, error) {
	return nil, nil
}

func (c *Client) HandleChatHistoryAfter(channel string, t time.Time,
	limit int) ([]ilayer.Message, error) {
	return nil, nil
}

func (c *Client) HandleChatHistoryLatest(channel string, after time.Time,
	limit int) ([]ilayer.Message, error) {
	return nil, nil
}

func (c *Client) HandleChatHistoryAround(channel string, t time.Time,
	limit int) ([]ilayer.Message, error) {
	return nil, nil
}

func (c *Client) HandleChatHistoryBetween(channel string, after time.Time,
	before time.Time, limit int) ([]ilayer.Message, error) {
	return nil, nil
}

func (c *Client) HandleTypingActive(channel string) error {
	c.session.Typing(c.guild, channel)
	return nil
}

func (c *Client) HandleTypingPaused(channel string) error {
	return nil
}

func (c *Client) HandleTypingDone(channel string) error {
	return nil
}
