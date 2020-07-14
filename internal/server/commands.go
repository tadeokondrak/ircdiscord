package server

import (
	"fmt"
	"strings"

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
	case "JOIN":
		return c.handleJoin(msg)
	case "PRIVMSG":
		return c.handlePrivmsg(msg)
	case "LIST":
		return c.handleList(msg)
	default:
		return nil
	}
}

func (c *Client) maybeCompleteRegistration() error {
	fmt.Println("maybecomplete")
	fmt.Println(c.isRegistered, c.nickname, c.username, c.realname, c.isCapBlocked)
	if c.isRegistered ||
		c.nickname == "" ||
		c.username == "" ||
		c.realname == "" ||
		c.isCapBlocked {
		return nil
	}
	fmt.Println("maybecompleteyes")

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
		if err := c.Server.HandleJoin(channel); err != nil {
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
