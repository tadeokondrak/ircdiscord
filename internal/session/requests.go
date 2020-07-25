package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
)

type validateGuild struct {
	guildID discord.GuildID
	err     chan<- error
}

type sendMessage struct {
	guildID discord.GuildID
	channel string
	content string
	err     chan<- error
}

type chatHistoryBefore struct {
	guildID discord.GuildID
	channel string
	t       time.Time
	limit   int
}

type typing struct {
	guildID discord.GuildID
	channel string
}

type typingSubscribe struct {
	guildID discord.GuildID
}

type userName struct {
	resp chan<- string
}

type nickName struct {
	guildID discord.GuildID
	resp    chan<- string
}

func (s *Session) handleRequest(req interface{}) error {
	switch req := req.(type) {
	case validateGuild:
		_, err := s.state.Guild(req.guildID)
		req.err <- err
	case sendMessage:
		channelID := s.names.ChannelID(req.guildID, req.channel)
		_, err := s.state.SendMessage(channelID, req.content, nil)
		req.err <- err
	case chatHistoryBefore:
		channelID := s.names.ChannelID(req.guildID, req.channel)
		msgs, err := s.state.MessagesBefore(channelID,
			discord.MessageID(discord.NewSnowflake(req.t)),
			uint(req.limit))
		if err != nil {
			return err
		}
		rmsgs := make([]MessageEvent, len(msgs))
		for i, msg := range msgs {
			var err error
			rmsgs[i], err = s.messageToEvent(&msg, req.guildID)
			if err != nil {
				return err
			}
		}
		s.broadcastGuild(&ChannelHistoryEvent{
			req.channel, rmsgs}, req.guildID)
		return nil
	case typing:
		channelID := s.names.ChannelID(req.guildID, req.channel)
		return s.state.Typing(channelID)
	case typingSubscribe:
		return s.state.Gateway.GuildSubscribe(
			gateway.GuildSubscribeData{
				Typing: true, GuildID: req.guildID})
	case channel:
		channelID := s.names.ChannelID(req.guildID, req.channelName)
		if !channelID.Valid() {
			return fmt.Errorf("channel %s not found", req.channelName)
		}

		_, err := s.state.Channel(channelID)
		req.err <- err

	case channelTopic:
		channelID := s.names.ChannelID(req.guildID, req.channelName)
		if !channelID.Valid() {
			req.err <- fmt.Errorf("unknown channel %s", req.channelName)
		}

		channel, err := s.state.Channel(channelID)
		if err != nil {
			req.err <- err
		}

		req.resp <- channel.Topic
	case userName:
		req.resp <- s.userName(s.userID)
	case nickName:
		req.resp <- s.nickName(req.guildID, s.userID)
	default:
		panic(fmt.Errorf("unexpected request %T", req))
	}
	return nil
}

// This function can be called concurrently from multiple goroutines.
func (s *Session) ValidateGuild(guildToken Guild) error {
	err := make(chan error)
	s.reqs <- validateGuild{
		discord.GuildID(discord.GuildID(guildToken)), err}
	return <-err
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) SendMessage(guildToken Guild, channelName string,
	content string) error {
	channelName = strings.TrimPrefix(channelName, "#")
	resp := make(chan error)
	s.reqs <- sendMessage{
		discord.GuildID(guildToken), channelName, content, resp}
	return <-resp
}

// This function can be called concurrently from multiple goroutines.
func (s *Session) UserID() int64 {
	return int64(s.userID)
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) UserName() string {
	resp := make(chan string)
	s.reqs <- userName{resp}
	return <-resp
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) NickName(guildToken Guild) string {
	resp := make(chan string)
	s.reqs <- nickName{discord.GuildID(guildToken), resp}
	return <-resp
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) GuildName(guildToken Guild) (string, error) {
	if guildToken == 0 {
		return "Discord", nil
	}

	guild, err := s.state.Guild(discord.GuildID(guildToken))
	if err != nil {
		return "", err
	}

	return guild.Name, nil
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) GuildDate(guildToken Guild) (time.Time, error) {
	if guildToken == 0 {
		return s.userID.Time(), nil
	}

	return discord.GuildID(guildToken).Time(), nil
}

type channel struct {
	guildID     discord.GuildID
	channelName string
	err         chan<- error
}

func (s *Session) Channel(guildToken Guild, channelName string) error {
	guildID := discord.GuildID(guildToken)
	channelName = strings.TrimPrefix(channelName, "#")
	err := make(chan error)
	s.reqs <- channel{guildID, channelName, err}
	return <-err
}

type channelTopic struct {
	guildID     discord.GuildID
	channelName string
	resp        chan<- string
	err         chan<- error
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) ChannelTopic(guildToken Guild,
	channelName string) (string, error) {
	resp := make(chan string)
	err := make(chan error)
	channelName = strings.TrimPrefix(channelName, "#")
	guildID := discord.GuildID(guildToken)
	s.reqs <- channelTopic{guildID, channelName, resp, err}
	select {
	case resp := <-resp:
		return resp, nil
	case err := <-err:
		return "", err
	}
}

// This function can be called from multiple concurrent goroutines.
func (s *Session) ChatHistoryBefore(guildToken Guild, channelName string,
	t time.Time, limit int) {
	channelName = strings.TrimPrefix(channelName, "#")
	guildID := discord.GuildID(guildToken)
	s.reqs <- chatHistoryBefore{guildID, channelName, t, limit}
}

func (s *Session) Typing(guildToken Guild, channelName string) {
	guildID := discord.GuildID(guildToken)
	channelName = strings.TrimPrefix(channelName, "#")
	s.reqs <- typing{guildID, channelName}
}

func (s *Session) TypingSubscribe(guildToken Guild) {
	guildID := discord.GuildID(guildToken)
	s.reqs <- typingSubscribe{guildID}
}
