package main

import (
	"sync"

	"sync/atomic"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/state"
)

type Session struct {
	*state.State
	id   discord.Snowflake
	refs uint32
}

var (
	ids         = make(map[string]discord.Snowflake)
	sessions    = make(map[discord.Snowflake]*Session)
	sessionLock sync.Mutex
)

func GetSession(token string) (*Session, error) {
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
		State: discord,
		refs:  0,
	}

	me, err := discord.Me()
	if err != nil {
		return nil, err
	}

	session.id = me.ID
	ids[token] = session.id
	sessions[session.id] = session

	return session, nil
}

func (s *Session) Ref() {
	atomic.AddUint32(&s.refs, 1)
}

func (s *Session) Unref() error {
	if atomic.AddUint32(&s.refs, ^uint32(0)) == 0 {
		sessionLock.Lock()
		defer sessionLock.Unlock()
		delete(sessions, s.Ready.User.ID)
		return s.Close()
	}
	return nil
}

func (s *Session) Close() error {
	return s.State.Close()
}
