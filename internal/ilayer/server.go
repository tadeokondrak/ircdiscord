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

	HandleJoin(channel string) (string, error)
	HandleTopic(channel string) (string, error)
	HandleCreationTime(channel string) (time.Time, error)
	HandleNames(channel string) ([]string, error)
	HandleMessage(channel, content string) error
	HandleList() ([]ListEntry, error)
	HandleWhois(user string) (WhoisReply, error)
	HandleChatHistoryBefore(channel string, t time.Time, limit int,
	) ([]Message, error)
	HandleChatHistoryAfter(channel string, t time.Time, limit int,
	) ([]Message, error)
	HandleChatHistoryLatest(channel string, after time.Time, limit int,
	) ([]Message, error)
	HandleChatHistoryAround(channel string, t time.Time, limit int,
	) ([]Message, error)
	HandleChatHistoryBetween(
		channel string, after time.Time, before time.Time, limit int,
	) ([]Message, error)
	HandleTypingActive(channel string) error
	HandleTypingPaused(channel string) error
	HandleTypingDone(channel string) error
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
