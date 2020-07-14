package replies

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gopkg.in/irc.v3"
)

type Writer interface {
	HasCapability(string) bool
	ClientPrefix() *irc.Prefix
	ServerPrefix() *irc.Prefix
	WriteMessage(*irc.Message) error
}

func CAP_LS(w Writer, supported []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "CAP",
		Params: []string{w.ClientPrefix().Name, "LS",
			strings.Join(supported, " ")},
	})
}

func CAP_LIST(w Writer, enabled []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "CAP",
		Params: []string{w.ClientPrefix().Name, "LIST",
			strings.Join(enabled, " ")},
	})
}

func CAP_ACK(w Writer, acked []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "CAP",
		Params: []string{w.ClientPrefix().Name, "ACK",
			strings.Join(acked, " ")},
	})
}

func NICK(w Writer, prefix *irc.Prefix, name string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  prefix,
		Command: "NICK",
		Params:  []string{name},
	})
}

func JOIN(w Writer, channel string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ClientPrefix(),
		Command: "JOIN",
		Params:  []string{channel},
	})
}

func PRIVMSG(w Writer, t time.Time, prefix *irc.Prefix, channel, message string) error {
	tags := make(irc.Tags)
	if w.HasCapability("server-time") && !t.IsZero() {
		tags["time"] =
			irc.TagValue(t.UTC().Format("2006-01-02T15:04:05.000Z"))
	}
	return w.WriteMessage(&irc.Message{
		Tags:    tags,
		Prefix:  prefix,
		Command: "PRIVMSG",
		Params:  []string{channel, message},
	})
}

func PONG(w Writer, param string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "PONG",
		Params:  []string{param},
	})
}

func NOTICE(w Writer, prefix *irc.Prefix, channel, message string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  prefix,
		Command: "NOTICE",
		Params:  []string{channel, message},
	})
}

func RPL_WELCOME(w Writer, towhat string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WELCOME,
		Params: []string{w.ClientPrefix().Name, fmt.Sprintf(
			"Welcome to %s, %s",
			towhat, w.ClientPrefix().Name)},
	})
}

func RPL_YOURHOST(w Writer, serverName, serverVersion string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_YOURHOST,
		Params: []string{w.ClientPrefix().Name, fmt.Sprintf(
			"Your host is %s, running version %s",
			serverName, serverVersion)},
	})
}

func RPL_CREATED(w Writer, t time.Time) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_CREATED,
		Params: []string{w.ClientPrefix().Name, fmt.Sprintf(
			"This server was created %v", t.UTC())},
	})
}

func RPL_TOPIC(w Writer, channel, topic string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_TOPIC,
		Params:  []string{w.ClientPrefix().Name, channel, topic},
	})
}

func RPL_NOTOPIC(w Writer, channel string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_TOPIC,
		Params: []string{w.ClientPrefix().Name, channel,
			"No topic is set"},
	})
}

func RPL_CREATIONTIME(w Writer, channel string, t time.Time) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "329",
		Params: []string{w.ClientPrefix().Name, channel,
			strconv.FormatInt(t.UTC().Unix(), 10)},
	})
}

func RPL_LISTSTART(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_LISTSTART,
		Params:  []string{w.ClientPrefix().Name, "Channel list"},
	})
}

func RPL_LIST(w Writer, channel string, visible int, topic string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_LIST,
		Params: []string{w.ClientPrefix().Name, channel,
			strconv.Itoa(visible), topic},
	})
}

func RPL_LISTEND(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_LISTEND,
		Params:  []string{w.ClientPrefix().Name, "End of /LIST"},
	})
}

func RPL_MOTDSTART(w Writer, serverName string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_MOTDSTART,
		Params: []string{w.ClientPrefix().Name,
			fmt.Sprintf("- %s Message of the day - ", serverName)},
	})
}

func RPL_MOTD(w Writer, line string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_MOTD,
		Params:  []string{w.ClientPrefix().Name, line},
	})
}

func RPL_ENDOFMOTD(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_ENDOFMOTD,
		Params:  []string{w.ClientPrefix().Name, "End of /MOTD command."},
	})
}

func ERR_NOMOTD(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.ERR_NOMOTD,
		Params:  []string{w.ClientPrefix().Name, "No MOTD"},
	})
}

func RPL_WHOISUSER(w Writer, prefix *irc.Prefix, realname string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WHOISUSER,
		Params: []string{w.ClientPrefix().Name, prefix.Name,
			prefix.User, prefix.Host, "*", realname},
	})
}

func RPL_WHOISSERVER(w Writer, user, server, serverInfo string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WHOISSERVER,
		Params: []string{w.ClientPrefix().Name, user,
			server, serverInfo},
	})
}

func RPL_WHOISOPERATOR(w Writer, user string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WHOISOPERATOR,
		Params: []string{w.ClientPrefix().Name,
			user, "is an IRC operator"},
	})
}

func RPL_WHOISIDLE(w Writer, user string, lastActive time.Time) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WHOISCHANNELS,
		Params: []string{w.ClientPrefix().Name, user,
			fmt.Sprint(int(lastActive.Sub(time.Now()).Seconds()))},
	})
}

func RPL_WHOISCHANNELS(w Writer, user string, channels []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_WHOISCHANNELS,
		Params: []string{w.ClientPrefix().Name,
			user, strings.Join(channels, " ")},
	})
}

func RPL_ENDOFWHOIS(w Writer, user string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_ENDOFWHOIS,
		Params: []string{w.ClientPrefix().Name,
			user, "End of /WHOIS list"},
	})
}
