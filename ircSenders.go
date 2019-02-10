package main

import (
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

func (c *ircConn) sendJOIN(nick string, realname string, hostname string, target string) (err error) {
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
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.JOIN,
		Params:  []string{target},
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

func (c *ircConn) sendPING(message string) (err error) {
	err = c.encode(&irc.Message{
		Command: irc.PING,
		Params:  []string{message},
	})
	c.lastPING = message
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

func (c *ircConn) sendPRIVMSG(tags irc.Tags, nick string, realname string, hostname string, target string, content string) (err error) {
	if content == "" {
		content = " "
	}
	_tags := irc.Tags{}

	// TODO: clean up
	if c.user.supportedCapabilities["server-time"] && tags["time"] != "" {
		_tags["time"] = tags["time"]
	}

	if c.user.supportedCapabilities["batch"] && tags["batch"] != "" {
		_tags["batch"] = tags["batch"]
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
	if len(_tags) == 0 {
		err = c.encode(&irc.Message{
			Prefix:  prefix,
			Command: irc.PRIVMSG,
			Params:  []string{target, content},
		})
	} else {
		err = c.encode(&irc.Message{
			Tags:    &_tags,
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

func (c *ircConn) sendBATCH(start bool, tag string, params ...string) (err error) {
	BATCH := "BATCH" // TODO: put in irc lib fork
	prefix := "+"
	if !start {
		prefix = "-"
	}

	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: BATCH,
		Params:  append([]string{prefix + tag}, params...),
	})
	return
}
