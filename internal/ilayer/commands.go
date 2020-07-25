package ilayer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tadeokondrak/ircdiscord/internal/replies"
	"gopkg.in/irc.v3"
)

func (c *Client) HandleMessage(msg *irc.Message) error {
	switch msg.Command {
	case "CAP":
		return c.handleCap(msg)
	case "PASS":
		return c.handlePass(msg)
	case "NICK":
		return c.handleNick(msg)
	case "USER":
		return c.handleUser(msg)
	case "PING":
		return c.handlePing(msg)
	case "AUTHENTICATE":
		return c.handleAuthenticate(msg)
	case "JOIN":
		return c.handleJoin(msg)
	case "PRIVMSG":
		return c.handlePrivmsg(msg)
	case "LIST":
		return c.handleList(msg)
	case "WHOIS":
		return c.handleWhois(msg)
	case "CHATHISTORY":
		return c.handleChatHistory(msg)
	case "TAGMSG":
		return c.handleTagmsg(msg)
	default:
		return nil
	}
}

func (c *Client) maybeCompleteRegistration() error {
	if c.isRegistered ||
		c.nickname == "" ||
		c.username == "" ||
		c.realname == "" ||
		c.isCapBlocked ||
		c.isAuthBlocked {
		return nil
	}

	if err := c.Server.HandleRegister(); err != nil {
		return err
	}

	c.isRegistered = true

	networkName, err := c.Server.NetworkName()
	if err != nil {
		return err
	}

	if err := replies.RPL_WELCOME(c, networkName); err != nil {
		return err
	}

	serverName, err := c.Server.ServerName()
	if err != nil {
		return err
	}

	serverVersion, err := c.Server.ServerVersion()
	if err != nil {
		return err
	}

	if err := replies.RPL_YOURHOST(
		c, serverName, serverVersion); err != nil {
		return err
	}

	serverCreated, err := c.Server.ServerCreated()
	if err != nil {
		return err
	}

	if err := replies.RPL_CREATED(
		c, serverCreated); err != nil {
		return err
	}

	if err := replies.RPL_MOTDSTART(
		c, serverName); err != nil {
		return err
	}

	motd, err := c.Server.MOTD()
	if err != nil {
		return err
	}

	for _, line := range motd {
		if err := replies.RPL_MOTD(c, line); err != nil {
			return err
		}
	}

	if err := replies.RPL_ENDOFMOTD(
		c); err != nil {
		return err
	}

	return nil
}

func (c *Client) handlePass(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	password, err := c.Server.HandlePassword(msg.Params[0])
	if err != nil {
		return err
	}

	c.password = password

	return c.maybeCompleteRegistration()
}

func (c *Client) handleAuthenticate(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	arg := msg.Params[0]
	if arg == "PLAIN" {
		return replies.AUTHENTICATE(c, "+")
	} else {
		if arg == "+" {
			arg = ""
		}

		c.saslProgress = append(c.saslProgress, []byte(arg)...)
		c.isAuthBlocked = true

		if len(arg) == 400 {
			return nil
		}

		c.isAuthBlocked = false

		decodedlen := base64.RawStdEncoding.DecodedLen(
			len(c.saslProgress))

		decoded := make([]byte, decodedlen)
		if _, err := base64.RawStdEncoding.Decode(
			decoded, c.saslProgress); err != nil {
			return err
		}

		args := bytes.Split(decoded, []byte{0})
		if len(args) != 3 {
			return fmt.Errorf("invalid sasl authenticate")
		}

		c.saslUsername = string(args[0])
		c.saslIdentity = string(args[1])
		c.saslPassword = string(args[2])

		if err := replies.RPL_LOGGEDIN(
			c, c.saslIdentity, c.username); err != nil {
			return err
		}

		if err := replies.RPL_SASLSUCCESS(c); err != nil {
			return err
		}
	}

	return c.maybeCompleteRegistration()
}

func (c *Client) handleNick(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	nickname, err := c.Server.HandleNickname(msg.Params[0])
	if err != nil {
		return nil
	}

	c.nickname = nickname

	return c.maybeCompleteRegistration()
}

func (c *Client) handleUser(msg *irc.Message) error {
	if err := checkParamCount(msg, 4, 4); err != nil {
		return err
	}

	if c.IsRegistered() {
		return fmt.Errorf("already registered")
	}

	username, err := c.Server.HandleUsername(msg.Params[0])
	if err != nil {
		return err
	}

	c.username = username

	realname, err := c.Server.HandleRealname(msg.Params[3])
	if err != nil {
		return err
	}

	c.realname = realname

	return c.maybeCompleteRegistration()
}

func (c *Client) handlePing(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	nonce, err := c.Server.HandlePing(msg.Params[0])
	if err != nil {
		return err
	}

	return replies.PONG(c, nonce)
}

func (c *Client) handleJoin(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	for _, channel := range strings.Split(msg.Params[0], ",") {
		if _, ok := c.channels[channel]; ok {
			continue // already in channel
		}

		channel, err := c.Server.HandleJoin(channel)
		if err != nil {
			c.channels[channel] = struct{}{}
			return err
		}

		if err := replies.JOIN(
			c, c.ClientPrefix(), channel); err != nil {
			return err
		}

		c.channels[channel] = struct{}{}

		if err != nil {
			return err
		}

		if topic, err := c.Server.HandleTopic(channel); err != nil {
			return err
		} else if topic != "" {
			if err := replies.RPL_TOPIC(
				c, channel, topic); err != nil {
				return err
			}
		} else {
			if err := replies.RPL_NOTOPIC(c, channel); err != nil {
				return err
			}
		}

		created, err := c.Server.HandleCreationTime(channel)
		if err != nil {
			return err
		}

		if !created.IsZero() {
			if err := replies.RPL_CREATIONTIME(
				c, channel, created); err != nil {
				return err
			}
		}

		names, err := c.Server.HandleNames(channel)
		if err != nil {
			return err
		}
		for _, name := range names {
			if err := replies.RPL_NAMREPLY(
				c, channel, name); err != nil {
				return err
			}
		}
		if err := replies.RPL_ENDOFNAMES(
			c, channel); err != nil {
			return err
		}

	}

	return nil
}

func (c *Client) handlePrivmsg(msg *irc.Message) error {
	if err := checkParamCount(msg, 2, 2); err != nil {
		return err
	}

	if err := c.Server.HandleMessage(
		msg.Params[0], msg.Params[1]); err != nil {
		return err
	}

	return nil
}

func (c *Client) handleList(msg *irc.Message) error {
	if err := checkParamCount(msg, 0, 2); err != nil {
		return err
	}

	list, err := c.Server.HandleList()
	if err != nil {
		return err
	}

	if err := replies.RPL_LISTSTART(c); err != nil {
		return err
	}

	for _, entry := range list {
		if err := replies.RPL_LIST(c,
			entry.Channel, entry.Users, entry.Topic); err != nil {
			return err
		}
	}

	if err := replies.RPL_LISTEND(c); err != nil {
		return err
	}

	return nil
}

func (c *Client) handleWhois(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	info, err := c.Server.HandleWhois(msg.Params[0])
	if err != nil {
		return err
	}

	if err := replies.RPL_WHOISUSER(
		c, info.Prefix, info.Realname); err != nil {
		return err
	}

	if info.Server != "" || info.ServerInfo != "" {
		if err := replies.RPL_WHOISSERVER(
			c, info.Prefix.Name,
			info.Server, info.ServerInfo); err != nil {
			return err
		}
	}

	if info.IsOperator {
		if err := replies.RPL_WHOISOPERATOR(c,
			info.Prefix.Name); err != nil {
			return err
		}
	}

	if !info.LastActive.IsZero() {
		if err := replies.RPL_WHOISIDLE(c,
			info.Prefix.Name, info.LastActive); err != nil {
			return err
		}
	}

	if err := replies.RPL_WHOISCHANNELS(c,
		info.Prefix.Name, info.Channels); err != nil {
		return err
	}

	if err := replies.RPL_ENDOFWHOIS(c, info.Prefix.Name); err != nil {
		return err
	}

	return nil
}

func parseTimestampOrMsgid(s string) (time.Time, string, bool, error) {

	if strings.HasPrefix(s, "timestamp=") {
		t, err := time.Parse("2006-01-02T15:04:05.000Z",
			strings.TrimPrefix(s, "timestamp="))
		return t, "", false, err
	} else if strings.HasPrefix(s, "msgid=") {
		return time.Time{}, strings.TrimPrefix(s, "msgid="), false, nil
	} else if s == "*" {
		return time.Time{}, "", true, nil
	} else {
		return time.Time{}, "", false, fmt.Errorf("unknown timestamp/msgid/* %s", s)
	}

}

func (c *Client) handleChatHistory(msg *irc.Message) error {
	if err := checkParamCount(msg, 4, 5); err != nil {
		return err
	}

	target := msg.Params[1]

	t, _, _, err := parseTimestampOrMsgid(msg.Params[2])
	if err != nil {
		return err
	}

	if target == "*" {
		return fmt.Errorf("CHATHISTORY target * not implemented")
	}

	limit, err := strconv.Atoi(msg.Params[3])
	if err != nil && msg.Params[0] != "BETWEEN" {
		return err
	}

	var msgs []Message
	switch msg.Params[0] {
	case "BEFORE":
		msgs, err = c.Server.HandleChatHistoryBefore(
			target, t, limit)
	case "AFTER":
		msgs, err = c.Server.HandleChatHistoryAfter(
			target, t, limit)
	case "LATEST":
		msgs, err = c.Server.HandleChatHistoryLatest(
			target, t, limit)
	case "AROUND":
		msgs, err = c.Server.HandleChatHistoryAround(
			target, t, limit)
	case "BETWEEN":
		if err := checkParamCount(msg, 5, 5); err != nil {
			return err
		}

		limit, err := strconv.Atoi(msg.Params[4])
		if err != nil {
			return err
		}

		t2, _, _, err := parseTimestampOrMsgid(msg.Params[3])
		if err != nil {
			return err
		}

		msgs, err = c.Server.HandleChatHistoryBetween(
			target, t, t2, limit)
	}
	if err != nil {
		return err
	}

	batch := c.nextBatch()

	if err := replies.BATCH(c, batch, "chathistory", target); err != nil {
		return err
	}

	for _, msg := range msgs {
		c.message(&msg, batch)
	}

	if err := replies.BATCH(c, batch); err != nil {
		return err
	}

	return nil
}

func (c *Client) handleTagmsg(msg *irc.Message) error {
	if err := checkParamCount(msg, 1, 1); err != nil {
		return err
	}

	if tag, ok := msg.Tags["+typing"]; ok {
		switch tag {
		case "active":
			return c.Server.HandleTypingActive(msg.Params[0])
		case "paused":
			return c.Server.HandleTypingPaused(msg.Params[0])
		case "done":
			return c.Server.HandleTypingDone(msg.Params[0])
		}
	}
	return nil
}
