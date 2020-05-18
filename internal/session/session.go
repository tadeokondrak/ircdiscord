package session

import (
	"sync"

	"sync/atomic"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/state"
	"github.com/tadeokondrak/ircdiscord/internal/idmap"
)

// Session is the reference-counted shared state between all Clients of a
// specific Discord user.
//
// It notably includes the Discord state, as well as a map of IRC nicks to
// Discord users.
type Session struct {
	*state.State
	NickMap *idmap.IDMap
	id      discord.Snowflake
	refs    uint32
}

var (
	ids         = make(map[string]discord.Snowflake)
	sessions    = make(map[discord.Snowflake]*Session)
	sessionLock sync.Mutex
)

// Get returns the Session for a given token, connecting to Discord if
// it does not already exist.
func Get(token string) (*Session, error) {
	sessionLock.Lock()
	defer sessionLock.Unlock()

	if id, ok := ids[token]; ok {
		if s, ok := sessions[id]; ok {
			s.Ref()
			return s, nil
		}
	}

	discord, err := state.New(token)
	if err != nil {
		return nil, err
	}

	if err := discord.Open(); err != nil {
		return nil, err
	}

	session := &Session{
		State:   discord,
		NickMap: idmap.New(),
		refs:    0,
	}
	session.NickMap.Concurrent = true

	me, err := discord.Me()
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
