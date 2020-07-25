package session

import (
	"log"
	"strings"
	"sync"
	"unicode"

	"sync/atomic"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/diamondburned/arikawa/state"
	"github.com/diamondburned/arikawa/utils/httputil/httpdriver"
	"github.com/diamondburned/ningen"
	"github.com/tadeokondrak/ircdiscord/internal/names"
)

// Session is the reference-counted shared state between all clients for a
// Discord user.
type Session struct {
	refs uint32

	// read-only after construction
	state      *ningen.State
	removeFunc RemoveFunc
	userID     discord.UserID
	debug      bool

	clientsMutex sync.Mutex
	clients      map[chan<- interface{}]Guild
	names        *names.Map

	reqs chan interface{}
}

// RemoveFunc is a function type used to remove a Session from some storage
// when Close is called.
type RemoveFunc func(s *Session)

// New creates a new Session.
//
// removeFunc will called on Close.
func New(token string, debug bool, removeFunc RemoveFunc) (*Session, error) {
	plain, err := state.New(token)
	if err != nil {
		return nil, err
	}

	state, err := ningen.FromState(plain)
	if err != nil {
		return nil, err
	}

	if debug {
		state.AddHandler(func(e interface{}) {
			log.Printf("<-d %T\n", e)
		})

		state.OnRequest = append(state.OnRequest,
			func(r httpdriver.Request) error {
				log.Printf("->d %s\n", r.GetPath())
				return nil
			},
		)
	}

	s := &Session{
		refs:       1,
		state:      state,
		removeFunc: removeFunc,
		debug:      debug,
		clients:    make(map[chan<- interface{}]Guild),
		names:      names.NewMap(),
		reqs:       make(chan interface{}),
	}

	if err := state.Open(); err != nil {
		return nil, err
	}

	if err := s.handleReadyEvent(&s.state.Ready); err != nil {
		return nil, err
	}

	s.userID = s.state.Ready.User.ID

	return s, nil
}

// Ref increments the reference count.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) Ref() {
	refs := atomic.AddUint32(&s.refs, 1)

	if s.debug {
		log.Printf("session %p: refs %d -> %d", s, refs-1, refs)
	}
}

// Unref decrements the reference count, calling Close if it hits zero.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) Unref() error {
	refs := atomic.AddUint32(&s.refs, ^uint32(0))

	if s.debug {
		log.Printf("session %p: refs %d -> %d", s, refs+1, refs)
	}

	if refs == 0 {
		return s.Close()
	}

	return nil
}

// Close calls the remove function given in New then closes the Discord
// connection.
//
// This function can be called from multiple concurrent goroutines (once).
func (s *Session) Close() error {
	s.removeFunc(s)
	return s.state.Close()
}

// Run listens for Discord events and broadcasts them to all listeners.
func (s *Session) Run() error {
	events := make(chan gateway.Event)
	cancel := s.state.AddHandler(events)
	defer cancel()

	for {
		select {
		case ev := <-events:
			if err := s.handleEvent(ev); err != nil {
				return nil
			}
		case req := <-s.reqs:
			if err := s.handleRequest(req); err != nil {
				return nil
			}
		}
	}
}

func (s *Session) updateName(userID discord.UserID, username string) {
	username = sanitizeIRCName(username)
	before, current := s.names.UpdateUser(userID, username)
	if before != current {
		s.broadcastGuildFunc(func(guildID discord.GuildID) interface{} {
			beforeNick, currentNick :=
				s.names.NickNameWithUserNameFallback(guildID, userID)
			return &UserUpdateEvent{
				Before: User{
					Nickname: beforeNick,
					Username: before,
					ID:       userID.String(),
				},
				Current: User{
					Nickname: currentNick,
					Username: current,
					ID:       userID.String(),
				},
			}
		})
	}
}

func (s *Session) updateNick(guildID discord.GuildID, userID discord.UserID,
	nick string, username string) {
	if nick == "" {
		nick = username
	}
	nick = sanitizeIRCName(nick)
	username = sanitizeIRCName(username)
	beforeUser, currentUser := s.names.UpdateUser(userID, username)
	beforeNick, currentNick := s.names.UpdateNick(guildID, userID, nick)
	if beforeUser != currentUser || beforeNick != currentNick {
		s.broadcastGuild(&UserUpdateEvent{
			Before: User{
				Nickname: beforeNick,
				Username: beforeUser,
				ID:       userID.String(),
			},
			Current: User{
				Nickname: currentNick,
				Username: currentUser,
				ID:       userID.String(),
			},
		}, guildID)
	}
}

func (s *Session) updateUserFromUser(user *discord.User) {
	s.updateName(user.ID, user.Username)
}

func (s *Session) updateNickFromMember(guildID discord.GuildID,
	member *discord.Member) {
	s.updateNick(guildID, member.User.ID, member.Nick, member.User.Username)
}

func (s *Session) updateChannel(guildID discord.GuildID,
	channel *discord.Channel) {
	name := sanitizeIRCName(channel.Name)
	before, current := s.names.UpdateChannel(guildID, channel.ID, name)
	if before != current {
		s.broadcastGuild(&ChannelUpdateEvent{
			Before: Channel{
				Name: before,
				ID:   channel.ID.String(),
			},
			Current: Channel{
				Name: before,
				ID:   channel.ID.String(),
			},
		}, guildID)
	}
}
func (s *Session) userName(userID discord.UserID) string {
	return s.names.UserName(userID)
}

func (s *Session) nickName(guildID discord.GuildID,
	userID discord.UserID) string {
	if guildID == discord.GuildID(0) {
		return s.names.UserName(userID)
	}
	currentUsername := s.names.UserName(userID)
	before, current := s.names.NickNameWithUserNameFallback(guildID, userID)
	if before != current {
		s.broadcastGuild(&UserUpdateEvent{
			Before: User{
				Nickname: before,
				Username: currentUsername,
				ID:       userID.String(),
			},
			Current: User{
				Nickname: current,
				Username: currentUsername,
				ID:       userID.String(),
			},
		}, guildID)
	}
	return current
}

func sanitizeIRCName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) ||
			unicode.IsNumber(r) {
			return r
		}
		switch r {
		case '_', '-', '{', '}', '[', ']', '\\', '`', '|':
			return r
		}
		return -1
	}, name)
}
