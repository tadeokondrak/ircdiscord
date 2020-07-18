package session

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"sync/atomic"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/handler"
	"github.com/diamondburned/arikawa/state"
	"github.com/diamondburned/ningen"
	"github.com/tadeokondrak/ircdiscord/internal/idmap"
)

// Session is the reference-counted shared state between all clients for a
// Discord user.
type Session struct {
	*ningen.State
	removeFunc       RemoveFunc
	internalHandler  *handler.Handler
	userMap          map[discord.Snowflake]string
	userMapMutex     sync.RWMutex
	nickMaps         map[discord.Snowflake]*idmap.IDMap
	nickMapsMutex    sync.RWMutex
	channelMaps      map[discord.Snowflake]*idmap.IDMap
	channelMapsMutex sync.RWMutex
	id               discord.Snowflake
	refs             uint32
}

// RemoveFunc is a function type used to remove a Session from some storage
// when Close is called.
type RemoveFunc func(s *Session)

// New creates a new Session.
// removeFunc is a function that will called on Close.
func New(token string, debug bool, removeFunc RemoveFunc) (*Session, error) {
	plain, err := state.New(token)
	if err != nil {
		return nil, err
	}

	state, err := ningen.FromState(plain)
	if err != nil {
		return nil, err
	}

	s := &Session{
		State:           state,
		removeFunc:      removeFunc,
		internalHandler: handler.New(),
		userMap:         make(map[discord.Snowflake]string),
		nickMaps:        make(map[discord.Snowflake]*idmap.IDMap),
		channelMaps:     make(map[discord.Snowflake]*idmap.IDMap),
		refs:            0,
	}

	state.AddHandler(s.onEventHarvest)

	if err := state.Open(); err != nil {
		return nil, err
	}

	me, err := state.Me()
	if err != nil {
		return nil, err
	}

	s.id = me.ID

	return s, nil
}

// Ref increments the reference count.
func (s *Session) Ref() {
	atomic.AddUint32(&s.refs, 1)
}

// Unref decrements the reference count, calling Close if it hits zero.
func (s *Session) Unref() error {
	if atomic.AddUint32(&s.refs, ^uint32(0)) == 0 {
		return s.Close()
	}
	return nil
}

// Close calls the remove function given in New then closes the Discord
// connection.
func (s *Session) Close() error {
	s.removeFunc(s)
	return s.State.Close()
}

// Run does nothing for now.
func (s *Session) Run() error {
	return nil
}

// Messages overrides (*ningen.State).Messages.
// It is a temporary hack to process the users and members in a message before
// posting it, to avoid messages being sent before joins.
func (s *Session) Messages(
	channelID discord.Snowflake) ([]discord.Message, error) {
	messages, err := s.State.Messages(channelID)
	if err == nil {
		s.harvestMessages(messages)
	}
	return messages, err
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
		s.internalHandler.Call(ev)
	}

	return post, nil
}

// UserFromName returns the Discord user for the given IRC nickname.
func (s *Session) UserFromName(guild discord.Snowflake,
	name string) discord.Snowflake {
	nickMap := s.nickMap(guild)
	return nickMap.Snowflake(name)
}

// While IsInitial is true, the callback will only be called in one goroutine.
// This function blocks until all events with IsInitial are sent.
func (s *Session) SubscribeUserList(guild discord.Snowflake,
	handler func(*UserNameChange)) (cancel func()) {
	nickMap := s.nickMap(guild)
	nickMap.Access(func(forward map[discord.Snowflake]string,
		backward map[string]discord.Snowflake) {
		var change UserNameChange
		change.GuildID = guild
		change.IsInitial = true
		for id, name := range forward {
			change.ID = id
			change.New = name
			handler(&change)
		}
		cancel = s.internalHandler.AddHandler(handler)
	})
	return
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
