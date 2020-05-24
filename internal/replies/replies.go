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
		Params:  []string{w.ClientPrefix().Name, "LS", strings.Join(supported, " ")},
	})
}

func CAP_ACK(w Writer, acked []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: "CAP",
		Params:  []string{w.ClientPrefix().Name, "ACK", strings.Join(acked, " ")},
	})
}

func JOIN(w Writer, channels []string) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ClientPrefix(),
		Command: "JOIN",
		Params:  []string{strings.Join(channels, ",")},
	})
}

func PRIVMSG(w Writer, t time.Time, prefix *irc.Prefix, channel, message string) error {
	tags := make(irc.Tags)
	if w.HasCapability("server-time") && !t.IsZero() {
		tags["time"] = irc.TagValue(t.UTC().Format("2006-01-02T15:04:05.000Z"))
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

func RPL_YOURHOST(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.RPL_YOURHOST,
		Params: []string{w.ClientPrefix().Name, fmt.Sprintf(
			"Your host is %s, running ircdiscord",
			w.ServerPrefix().Name)},
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

func ERR_NOMOTD(w Writer) error {
	return w.WriteMessage(&irc.Message{
		Prefix:  w.ServerPrefix(),
		Command: irc.ERR_NOMOTD,
		Params:  []string{w.ClientPrefix().Name, "No MOTD"},
	})
}
