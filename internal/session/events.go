package session

import (
	"log"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/k0kubun/pp"
)

type Guild int64

type User struct {
	Nickname string
	Username string
	ID       string
}

type Channel struct {
	Name  string
	Topic string
	ID    string
}

type MessageEvent struct {
	Date    time.Time
	ID      string
	Channel string
	User    User
	Content string
}

type ChannelHistoryEvent struct {
	Channel string
	Events  []MessageEvent
}

type TypingEvent struct {
	Date    time.Time
	Channel string
	User    User
}

type UserUpdateEvent struct {
	Before  User
	Current User
}

type ChannelUpdateEvent struct {
	Before  Channel
	Current Channel
}

// Register adds ch to the listener list.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) Register(ch chan<- interface{}, guildToken Guild) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	s.clients[ch] = guildToken
}

// Unregister removes ch from the listener list,
// preventing it from receiving any more events.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) Unregister(ch chan<- interface{}) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	delete(s.clients, ch)
}

// broadcast broadcasts ev to all listening channels.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) broadcast(ev interface{}) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	pp.Println(ev)
	for ch := range s.clients {
		nonBlockingSend(ch, ev)
	}
}

// broadcastGuild broadcasts ev to all listening channels for guildID.
//
// This function can be called from multiple concurrent goroutines.
func (s *Session) broadcastGuild(ev interface{}, guildID discord.GuildID) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	for ch, id := range s.clients {
		if guildID == discord.GuildID(id) {
			nonBlockingSend(ch, ev)
		}
	}
}

func (s *Session) broadcastGuildFunc(evFunc func(discord.GuildID) interface{}) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()
	for ch, id := range s.clients {
		ev := evFunc(discord.GuildID(id))
		nonBlockingSend(ch, ev)
	}
}

func nonBlockingSend(ch chan<- interface{}, ev interface{}) {
	select {
	case ch <- ev:
		return
	case <-time.After(1 * time.Second):
		pp.Println(ev)
		log.Println("chan sending blocked!")
	}
}
