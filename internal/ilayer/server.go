package ilayer

import (
	"time"

	"gopkg.in/irc.v3"
)

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
