package main

import (
	"time"

	"github.com/tadeokondrak/irc"
)

func (c *ircConn) sendNOTICE(message string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.NOTICE,
		Params:  append([]string{c.clientPrefix.Name}, message),
	})
	return
}
func (c *ircConn) sendERR(command string, params ...string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: command,
		Params:  append([]string{c.clientPrefix.Name}, params...),
	})
	return
}

func (c *ircConn) sendRPL(command string, params ...string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: command,
		Params:  append([]string{c.clientPrefix.Name}, params...),
	})
	return
}

func (c *ircConn) sendJOIN(nick string, realname string, hostname string, target string, key string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.clientPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	params := []string{target}
	if key != "" {
		params = append(params, key)
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.JOIN,
		Params:  params,
	})
	return
}

func (c *ircConn) sendPART(nick string, realname string, hostname string, target string, reason string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.clientPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	params := []string{target}
	if reason != "" {
		params = append(params, reason)
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.PART,
		Params:  params,
	})
	return
}

func (c *ircConn) sendNICK(nick string, realname string, hostname string, newNick string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.serverPrefix

	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.NICK,
		Params:  []string{newNick},
	})
	return
}

func (c *ircConn) sendPONG(message string) (err error) {
	params := []string{}
	if message != "" {
		params = append(params, message)
	}
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.PONG,
		Params:  params,
	})
	return
}

func (c *ircConn) sendPRIVMSG(date time.Time, nick string, realname string, hostname string, target string, content string) (err error) {
	var tags irc.Tags
	if c.user.supportedCapabilities["server-time"] {
		tags = irc.Tags{}
		tags["time"] = date.Format("2006-01-02T15:04:05.000Z")
	}

	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.serverPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	if tags == nil {
		err = c.encode(&irc.Message{
			Prefix:  prefix,
			Command: irc.PRIVMSG,
			Params:  []string{target, content},
		})
	} else {
		err = c.encode(&irc.Message{
			Tags:    &tags,
			Prefix:  prefix,
			Command: irc.PRIVMSG,
			Params:  []string{target, content},
		})
	}
	return
}

func (c *ircConn) sendCAP(subcommand string, params ...string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.CAP,
		Params:  append([]string{c.user.nick, subcommand}, params...),
	})
	return
}
