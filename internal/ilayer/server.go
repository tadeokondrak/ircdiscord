package ilayer

import (
	"strings"
	"time"

	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"gopkg.in/irc.v3"
)

type Client struct {
	Server       Server
	Conn         *irc.Conn
	serverPrefix *irc.Prefix
	clientPrefix *irc.Prefix
	capabilities map[string]bool
	channels     map[string]bool
	nickname     string
	username     string
	realname     string
	password     string
	isRegistered bool
	isCapBlocked bool
}

func NewClient(conn *irc.Conn, serverAddr, clientAddr string) *Client {
	c := &Client{
		Conn:         conn,
		serverPrefix: &irc.Prefix{Name: serverAddr},
		clientPrefix: &irc.Prefix{Name: clientAddr},
		capabilities: make(map[string]bool),
		channels:     make(map[string]bool),
	}

	return c
}

func (c *Client) HasCapability(capability string) bool {
	return c.capabilities[capability]
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

func (c *Client) SetPassword(password string) error {
	panic("todo")
}

func (c *Client) IsRegistered() bool {
	return c.isRegistered
}

func (c *Client) InChannel(channel string) bool {
	return c.channels[channel]
}

func (c *Client) Join(channel, topic string, created time.Time) error {
	if err := replies.JOIN(c, channel); err != nil {
		return err
	}

	c.channels[channel] = true

	if topic != "" {
		if err := replies.RPL_TOPIC(c, channel, topic); err != nil {
			return err
		}
	}

	if err := replies.RPL_CREATIONTIME(c, channel, created); err != nil {
		return err
	}

	return nil
}

func (c *Client) Message(channel, content string, author *irc.Prefix,
	time time.Time) error {
	for _, line := range strings.Split(content, "\n") {
		if err := replies.PRIVMSG(
			c, time, author, channel, line,
		); err != nil {
			return err
		}
	}
	return nil
}

type Server interface {
	// Functions are not called before registration unless otherwise stated

	NetworkName() (string, error)
	ServerName() (string, error)
	ServerVersion() (string, error)
	ServerCreated() (time.Time, error)
	MOTD() ([]string, error)

	HandleNickname(nickname string) (string, error) // During registration
	HandleUsername(username string) (string, error) // During registration
	HandleRealname(realname string) (string, error) // During registration
	HandlePassword(password string) (string, error) // During registration
	HandlePing(nonce string) (string, error)        // During registration
	HandleRegister() error                          // During registration

	HandleJoin(channel string) error
	HandleMessage(channel, content string) error
	HandleList() ([]ListEntry, error)
	HandleWhois(user string) (WhoisReply, error)
}

type ListEntry struct {
	Channel string
	Users   int
	Topic   string
}

type WhoisReply struct {
	Prefix     *irc.Prefix
	Realname   string
	Server     string
	ServerInfo string
	IsOperator bool
	LastActive time.Time
	Channels   []string
}
