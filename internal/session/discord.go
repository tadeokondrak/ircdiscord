package session

import (
	"log"

	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
	"github.com/diamondburned/arikawa/state"
	"github.com/tadeokondrak/ircdiscord/internal/render"
)

func (s *Session) handleEvent(ev gateway.Event) error {
	switch ev := ev.(type) {
	case *gateway.ReadyEvent:
		return nil // handled in New
	case *gateway.MessageCreateEvent:
		return s.handleMessageCreateEvent(ev)
	case *gateway.MessageUpdateEvent:
		return s.handleMessageUpdateEvent(ev)
	case *gateway.PresenceUpdateEvent:
		return s.handlePresenceUpdateEvent(ev)
	case *gateway.TypingStartEvent:
		return s.handleTypingStartEvent(ev)
	case *state.GuildReadyEvent:
		return s.handleGuildReadyEvent(ev)
	default:
		log.Printf("unhandled event type: %T", ev)
		return nil
	}
}

func (s *Session) handleReadyEvent(ev *gateway.ReadyEvent) error {
	for _, ch := range ev.PrivateChannels {
		for _, user := range ch.DMRecipients {
			s.updateUserFromUser(&user)
		}
	}

	for _, gld := range ev.Guilds {
		for _, ch := range gld.Channels {
			s.updateChannel(gld.ID, &ch)
		}

		for _, mbr := range gld.Members {
			s.updateUserFromUser(&mbr.User)
			s.updateNickFromMember(gld.ID, &mbr)
		}
	}

	return nil
}

func (s *Session) handleMessageCreateEvent(ev *gateway.MessageCreateEvent) error {
	s.updateUserFromUser(&ev.Author)
	if ev.Member != nil {
		s.updateNickFromMember(ev.GuildID, ev.Member)
	}

	outEv, err := s.messageToEvent(&ev.Message, ev.GuildID)
	if err != nil {
		return err
	}

	s.broadcastGuild(&outEv, ev.GuildID)

	return nil
}

func (s *Session) handleMessageUpdateEvent(ev *gateway.MessageUpdateEvent) error {
	return nil
}

func (s *Session) handlePresenceUpdateEvent(ev *gateway.PresenceUpdateEvent) error {
	if ev.Nick != "" {
		s.updateNick(ev.GuildID, ev.User.ID, ev.Nick, ev.User.Username)
	}
	return nil
}

func (s *Session) handleTypingStartEvent(ev *gateway.TypingStartEvent) error {
	if ev.Member != nil {
		s.updateUserFromUser(&ev.Member.User)
		s.updateNickFromMember(ev.GuildID, ev.Member)
	}

	s.broadcastGuild(&TypingEvent{
		Date:    ev.Timestamp.Time(),
		Channel: s.names.ChannelName(ev.GuildID, ev.ChannelID),
		User: User{
			Nickname: s.nickName(ev.GuildID, ev.UserID),
			Username: s.userName(ev.UserID),
			ID:       ev.UserID.String(),
		},
	}, ev.GuildID)
	return nil
}

func (s *Session) handleGuildReadyEvent(ev *state.GuildReadyEvent) error {
	for _, ch := range ev.Channels {
		s.updateChannel(ev.ID, &ch)
	}

	for _, mbr := range ev.Members {
		s.updateUserFromUser(&mbr.User)
		s.updateNickFromMember(ev.ID, &mbr)
	}

	return nil
}

func (s *Session) messageToEvent(msg *discord.Message,
	guildID discord.GuildID) (MessageEvent, error) {
	content, err := render.Message(guildID, msg)
	if err != nil {
		return MessageEvent{}, err
	}

	s.updateUserFromUser(&msg.Author)

	return MessageEvent{
		Date:    msg.ID.Time(),
		ID:      msg.ID.String(),
		Channel: s.names.ChannelName(guildID, msg.ChannelID),
		User: User{
			Nickname: s.nickName(guildID, msg.Author.ID),
			Username: s.userName(msg.Author.ID),
			ID:       msg.Author.ID.String(),
		},
		Content: content,
	}, nil
}
