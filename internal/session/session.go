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
	"github.com/diamondburned/arikawa/handler"
	"github.com/diamondburned/arikawa/state"
	"github.com/diamondburned/arikawa/utils/httputil/httpdriver"
	"github.com/diamondburned/ningen"
	"github.com/tadeokondrak/ircdiscord/internal/idmap"
)

// Session is the reference-counted shared state between all clients for a
// Discord user.
type Session struct {
	*ningen.State
	SessionHandler   *handler.Handler
	userMap          map[discord.Snowflake]string
	userMapMutex     sync.RWMutex
	nickMaps         map[discord.Snowflake]*idmap.IDMap
	nickMapsMutex    sync.RWMutex
	channelMaps      map[discord.Snowflake]*idmap.IDMap
	channelMapsMutex sync.RWMutex
	id               discord.Snowflake
	refs             uint32
}

var (
	ids         = make(map[string]discord.Snowflake)
	sessions    = make(map[discord.Snowflake]*Session)
	sessionLock sync.Mutex
)

func (s *Session) Messages(
	channelID discord.Snowflake) ([]discord.Message, error) {
	messages, err := s.State.Messages(channelID)
	if err == nil {
		s.harvestMessages(messages)
	}
	return messages, err
}

// Get returns the Session for a given token, connecting to Discord if
// it does not already exist. If the session does not already exist, and
// debug is true, the newly created session will log information to stderr.
func Get(token string, enableDebug bool) (*Session, error) {
	sessionLock.Lock()
	defer sessionLock.Unlock()

	if id, ok := ids[token]; ok {
		if s, ok := sessions[id]; ok {
			s.Ref()
			return s, nil
		}
	}

	plainstate, err := state.New(token)
	if err != nil {
		return nil, err
	}

	state, err := ningen.FromState(plainstate)
	if err != nil {
		return nil, err
	}
	state.PreHandler = handler.New()
	state.PreHandler.Synchronous = false

	if enableDebug {
		state.AddHandler(func(e interface{}) {
			fmt.Printf("<-d %T\n", e)
		})
		state.OnRequest = append(state.OnRequest,
			func(r httpdriver.Request) error {
				fmt.Printf("->d %s\n", r.GetPath())
				return nil
			},
		)
	}

	events, cancel := state.ChanFor(func(e interface{}) bool {
		_, ok := e.(*gateway.ReadyEvent)
		return ok
	})
	defer cancel()

	session := &Session{
		State:          state,
		SessionHandler: handler.New(),
		userMap:        make(map[discord.Snowflake]string),
		nickMaps:       make(map[discord.Snowflake]*idmap.IDMap),
		channelMaps:    make(map[discord.Snowflake]*idmap.IDMap),
		refs:           0,
	}
	state.PreHandler.AddHandler(session.onEventHarvest)

	if err := state.Open(); err != nil {
		return nil, err
	}

	<-events

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

// Close closes the Discord connection. This should generally not be called,
// since Unref closes the connection on last disconnect.
func (s *Session) Close() error {
	return s.State.Close()
}

func safeGetMap(maps map[discord.Snowflake]*idmap.IDMap,
	id discord.Snowflake, mu *sync.RWMutex) *idmap.IDMap {
	mu.RLock()
	m, ok := maps[id]
	mu.RUnlock()
	if ok {
		return m
	}

	mu.Lock()
	maps[id] = idmap.New()
	mu.Unlock()
	return maps[id]
}

func (s *Session) nickMap(guild discord.Snowflake) *idmap.IDMap {
	return safeGetMap(s.nickMaps, guild, &s.nickMapsMutex)
}

func (s *Session) channelMap(guild discord.Snowflake) *idmap.IDMap {
	return safeGetMap(s.channelMaps, guild, &s.channelMapsMutex)
}

func (s *Session) plainUsername(userID discord.Snowflake) (string, error) {
	s.userMapMutex.RLock()
	name, ok := s.userMap[userID]
	s.userMapMutex.RUnlock()
	if ok {
		return name, nil
	}

	user, err := s.User(userID)
	if err != nil {
		return "", err
	}
	s.harvestUser(user)

	return user.Username, nil
}

var ErrInvalidSnowflake = errors.New("invalid snowflake given")

// UserName returns the IRC nickname for the given Discord user.
func (s *Session) UserName(guild discord.Snowflake,
	id discord.Snowflake) (string, error) {
	if !id.Valid() {
		return "", ErrInvalidSnowflake
	}

	nickMap := s.nickMap(guild)

	if name := nickMap.Name(id); name != "" {
		return name, nil
	}

	var name string
	if guild.Valid() {
		member, err := s.Store.Member(guild, id)
		if err == state.ErrStoreNotFound {
			var err error
			name, err = s.plainUsername(id)
			if err != nil {
				return "", err
			}
			s.MemberState.RequestMember(guild, id)
			goto insert
		} else if err != nil {
			return "", err
		}
		if member.Nick != "" {
			name = member.Nick
		} else {
			name = member.User.Username
		}
	} else {
		var err error
		name, err = s.plainUsername(id)
		if err != nil {
			return "", err
		}
	}

insert:
	pre, post := nickMap.Insert(id, sanitizeNick(name))
	if pre != post {
		ev := &UserNameChange{
			GuildID: guild,
			ID:      id,
			Old:     pre,
			New:     post,
		}
		s.SessionHandler.Call(ev)
	}

	return post, nil
}

// UserFromName returns the Discord user for the given IRC nickname.
func (s *Session) UserFromName(guild discord.Snowflake,
	name string) discord.Snowflake {
	nickMap := s.nickMap(guild)
	return nickMap.Snowflake(name)
}

var ErrNoChannel = errors.New("no channel by that name exists")

// ChannelFromName returns the Discord channel for a given IRC channel name.
func (s *Session) ChannelFromName(guild discord.Snowflake,
	name string) discord.Snowflake {
	channelMap := s.channelMap(guild)
	name = strings.TrimPrefix(name, "#")
	return channelMap.Snowflake(name)
}

// ChannelName returns the IRC channel name for the given Discord channel ID.
func (s *Session) ChannelName(guild discord.Snowflake,
	id discord.Snowflake) (string, error) {
	channelMap := s.channelMap(guild)

	if name := channelMap.Name(id); name != "" {
		return fmt.Sprintf("#%s", name), nil
	}

	channel, err := s.State.Channel(id)
	if err != nil {
		return "", err
	}

	// TODO: send event when pre != post
	_, post := channelMap.Insert(channel.ID, channel.Name)
	return fmt.Sprintf("#%s", post), nil
}

// sanitizeNick removes characters invalid in an IRC nickname from a string.
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
