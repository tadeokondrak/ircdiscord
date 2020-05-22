package session

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"sync/atomic"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/diamondburned/arikawa/state"
	"github.com/diamondburned/arikawa/utils/httputil/httpdriver"
	"github.com/tadeokondrak/ircdiscord/internal/idmap"
)

// Session is the reference-counted shared state between all Clients of a
// specific Discord user.
//
// It notably includes the Discord state, as well as a map of IRC nicks to
// Discord users.
type Session struct {
	*state.State
	NickMap          *idmap.IDMap
	ChannelMaps      map[discord.Snowflake]*idmap.IDMap
	ChannelMapsMutex sync.RWMutex
	id               discord.Snowflake
	refs             uint32
	debug            bool
}

var (
	ids         = make(map[string]discord.Snowflake)
	sessions    = make(map[discord.Snowflake]*Session)
	sessionLock sync.Mutex
)

// Get returns the Session for a given token, connecting to Discord if
// it does not already exist. If the session does not already exist, and
// debug is true, the newly created session will log information to stderr.
func Get(token string, debug bool) (*Session, error) {
	sessionLock.Lock()
	defer sessionLock.Unlock()

	if id, ok := ids[token]; ok {
		if s, ok := sessions[id]; ok {
			s.Ref()
			return s, nil
		}
	}

	state, err := state.New(token)
	if err != nil {
		return nil, err
	}

	if debug {
		state.AddHandler(func(e interface{}) { fmt.Printf("<-d %T\n", e) })
		state.OnRequest = append(state.OnRequest, func(r httpdriver.Request) error {
			fmt.Printf("d-> %s\n", r.GetPath())
			return nil
		})
	}

	events, cancel := state.ChanFor(func(e interface{}) bool { _, ok := e.(*gateway.ReadyEvent); return ok })
	defer cancel()

	if err := state.Open(); err != nil {
		return nil, err
	}

	<-events

	session := &Session{
		State:       state,
		NickMap:     idmap.New(),
		ChannelMaps: make(map[discord.Snowflake]*idmap.IDMap),
		refs:        0,
	}
	session.NickMap.Concurrent = true

	me, err := state.Me()
	if err != nil {
		return nil, err
	}

	session.id = me.ID
	ids[token] = session.id
	sessions[session.id] = session

	return session, nil
}

// Ref increases the reference count.
func (s *Session) Ref() {
	atomic.AddUint32(&s.refs, 1)
}

// Unref decreases the reference count, making the Session invalid if it's
// the last.
func (s *Session) Unref() error {
	if atomic.AddUint32(&s.refs, ^uint32(0)) == 0 {
		sessionLock.Lock()
		defer sessionLock.Unlock()
		delete(sessions, s.Ready.User.ID)
		return s.Close()
	}
	return nil
}

// Close closes the Discord connection. This should not generally be called,
// since Unref closes the connection on last disconnect.
func (s *Session) Close() error {
	return s.State.Close()
}

// UserName returns the IRC name for the given Discord user.
func (s *Session) UserName(d *discord.User) string {
	if !d.ID.Valid() {
		return sanitizeNick(d.Username)
	}
	return s.NickMap.Insert(d.ID, sanitizeNick(d.Username))
}

var ErrNoChannel = errors.New("no channel by that name exists")

func (s *Session) ChannelMap(guild discord.Snowflake) *idmap.IDMap {
	s.ChannelMapsMutex.RLock()
	themap, ok := s.ChannelMaps[guild]
	s.ChannelMapsMutex.RUnlock()
	if ok {
		return themap
	}

	s.ChannelMapsMutex.Lock()
	defer s.ChannelMapsMutex.Unlock()
	s.ChannelMaps[guild] = idmap.New()
	s.ChannelMaps[guild].Concurrent = true
	return s.ChannelMaps[guild]
}

// ChannelFromName returns the Discord channel for a given IRC channel name.
func (s *Session) ChannelFromName(guild discord.Snowflake, name string) discord.Snowflake {
	channelMap := s.ChannelMap(guild)
	return channelMap.Snowflake(strings.TrimPrefix(name, "#"))
}

// ChannelName returns the IRC channel name for the given Discord channel ID.
// It includes the leading #.
func (s *Session) ChannelName(guild discord.Snowflake, id discord.Snowflake) (string, error) {
	channelMap := s.ChannelMap(guild)

	if name := channelMap.Name(id); name != "" {
		return fmt.Sprintf("#%s", name), nil
	}

	channel, err := s.State.Channel(id)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("#%s", channelMap.Insert(channel.ID, channel.Name)), nil
}

// sanitizeNick removes invalid-for-IRC-nick characters from a string.
func sanitizeNick(s string) string {
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
	}, s)
}
