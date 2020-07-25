package ilayer

import (
	"fmt"
	"strings"
	"time"

	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"gopkg.in/irc.v3"
)

type Client struct {
	Server        Server
	Conn          *irc.Conn
	serverPrefix  *irc.Prefix
	clientPrefix  *irc.Prefix
	capabilities  map[string]struct{}
	channels      map[string]struct{}
	nickname      string
	username      string
	realname      string
	password      string
	saslUsername  string
	saslIdentity  string
	saslPassword  string
	saslProgress  []byte
	isRegistered  bool
	isCapBlocked  bool
	isAuthBlocked bool
	batchSerial   int
}

func NewClient(conn *irc.Conn, serverAddr, clientAddr string) *Client {
	c := &Client{
		Conn:         conn,
		serverPrefix: &irc.Prefix{Name: serverAddr},
		clientPrefix: &irc.Prefix{Name: clientAddr},
		capabilities: make(map[string]struct{}),
		channels:     make(map[string]struct{}),
	}

	return c
}

func (c *Client) HasCapability(capability string) bool {
	_, ok := c.capabilities[capability]
	return ok
}

func (c *Client) ClientPrefix() *irc.Prefix {
	return c.clientPrefix
}

// Can only be called before registration completes
func (c *Client) SetClientPrefix(prefix *irc.Prefix) {
	if !c.isRegistered {
		c.clientPrefix = prefix
	}
}

func (c *Client) ServerPrefix() *irc.Prefix {
	return c.serverPrefix
}

func (c *Client) SetServerPrefix(prefix *irc.Prefix) {
	c.serverPrefix = prefix
}

func (c *Client) ReadMessage() (*irc.Message, error) {
	return c.Conn.ReadMessage()
}

func (c *Client) WriteMessage(m *irc.Message) error {
	return c.Conn.WriteMessage(m)
}

func (c *Client) Nickname() string {
	return c.nickname
}

func (c *Client) SetNickname(nickname string) error {
	if err := replies.NICK(c, c.clientPrefix, nickname); err != nil {
		return err
	}
	c.clientPrefix.Name = nickname
	return nil
}

func (c *Client) Username() string {
	return c.username
}

func (c *Client) Realname() string {
	return c.realname
}

func (c *Client) Password() string {
	return c.password
}

func (c *Client) SASLUsername() string {
	return c.saslUsername
}

func (c *Client) SASLIdentity() string {
	return c.saslIdentity
}

func (c *Client) SASLPassword() string {
	return c.saslPassword
}

func (c *Client) IsRegistered() bool {
	return c.isRegistered
}

func (c *Client) nextBatch() string {
	c.batchSerial++
	return fmt.Sprint(c.batchSerial)
}

func (c *Client) InChannel(channel string) bool {
	_, ok := c.channels[channel]
	return ok
}

func (c *Client) Channels() []string {
	channels := []string{}
	for channel := range c.channels {
		channels = append(channels, channel)
	}
	return channels
}

func (c *Client) message(msg *Message, batch string) error {
	for _, line := range strings.Split(msg.Content, "\n") {
		if err := replies.PRIVMSG(
			c, msg.Date, msg.Author, msg.Channel,
			line, msg.ID, batch,
		); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Message(msg *Message) error {
	return c.message(msg, "")
}

func (c *Client) Typing(t time.Time, author *irc.Prefix, channel string) error {
	return replies.TAGMSGTyping(c, t, author, channel)
}

func (c *Client) TypingStop(t time.Time, author *irc.Prefix, channel string) error {
	return replies.TAGMSGTyping(c, t, author, channel)
}

type Message struct {
	Channel string
	Content string
	ID      string
	Author  *irc.Prefix
	Date    time.Time
}
