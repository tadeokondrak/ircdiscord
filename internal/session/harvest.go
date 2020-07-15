package session

import (
	"github.com/diamondburned/arikawa/discord"
	"github.com/diamondburned/arikawa/gateway"
)

func (s *Session) harvestUsername(user discord.Snowflake, name string) {
	if !user.Valid() {
		return
	}

	if name == "" {
		panic("valid id but empty username")
	}

	s.harvestNick(discord.Snowflake(0), user, name, name)

	s.userMapMutex.RLock()
	foundName, ok := s.userMap[user]
	s.userMapMutex.RUnlock()

	if ok && foundName == name {
		return
	}

	s.userMapMutex.Lock()
	s.userMap[user] = name
	s.userMapMutex.Unlock()
}

func (s *Session) harvestNick(guild, user discord.Snowflake, nick, username string) {
	if !user.Valid() {
		return
	}

	if nick == "" {
		nick = username
	}

	if nick == "" {
		panic("empty nick and username")
	}

	if user.Valid() && username == "" {
		panic("valid user id but empty username ")
	}

	s.nickMap(guild).InsertWithChangeCallback(user, sanitizeNick(nick),
		func(pre, post string) {
			ev := &UserNameChange{
				GuildID: guild,
				ID:      user,
				Old:     pre,
				New:     post,
			}
			s.SessionHandler.Call(ev)
		})
}

func (s *Session) harvestUser(user *discord.User) {
	s.harvestUsername(user.ID, user.Username)
}

func (s *Session) harvestUsers(users []discord.User) {
	for _, user := range users {
		s.harvestUser(&user)
	}
}

func (s *Session) harvestMember(guild discord.Snowflake, member *discord.Member) {
	if member == nil {
		return
	}

	s.harvestUser(&member.User)
	s.harvestNick(guild, member.User.ID, member.Nick, member.User.Username)
}

func (s *Session) harvestMembers(guild discord.Snowflake, members []discord.Member) {
	for _, member := range members {
		s.harvestMember(guild, &member)
	}
}

func (s *Session) harvestChannel(channel *discord.Channel) {
	s.harvestUsers(channel.DMRecipients)
}

func (s *Session) harvestChannels(channels []discord.Channel) {
	for _, channel := range channels {
		s.harvestChannel(&channel)
	}
}

func (s *Session) harvestGuild(guild *discord.Guild) {
}

func (s *Session) harvestGuilds(guilds []discord.Guild) {
	for _, guild := range guilds {
		s.harvestGuild(&guild)
	}
}

func (s *Session) harvestGuildCreateEvent(event *gateway.GuildCreateEvent) {
	s.harvestGuild(&event.Guild)
	s.harvestMembers(event.Guild.ID, event.Members)
	s.harvestChannels(event.Channels)
	s.harvestPresences(event.Presences)
}

func (s *Session) harvestGuildCreateEvents(events []gateway.GuildCreateEvent) {
	for _, event := range events {
		s.harvestGuildCreateEvent(&event)
	}
}

func (s *Session) harvestRelationship(relationship *discord.Relationship) {
	s.harvestUser(&relationship.User)
}

func (s *Session) harvestRelationships(relationships []discord.Relationship) {
	for _, relationship := range relationships {
		s.harvestRelationship(&relationship)
	}
}

func (s *Session) harvestMessage(message *discord.Message) {
	s.harvestUser(&message.Author)
	for _, guildUser := range message.Mentions {
		s.harvestMember(message.GuildID, guildUser.Member)
	}
}

func (s *Session) harvestMessages(messages []discord.Message) {
	for _, message := range messages {
		s.harvestMessage(&message)
	}
}

func (s *Session) harvestPresence(presence *discord.Presence) {
	if presence.User.Username != "" {
		s.harvestUser(&presence.User)
	}
	if presence.User.Username != "" && presence.Nick != "" {
		s.harvestNick(presence.GuildID, presence.User.ID,
			presence.Nick, presence.User.Username)
	}
}

func (s *Session) harvestPresences(presences []discord.Presence) {
	for _, presence := range presences {
		s.harvestPresence(&presence)
	}
}

func (s *Session) onEventHarvest(e interface{}) {
	switch e := e.(type) {
	case *gateway.HelloEvent:
	case *gateway.ReadyEvent:
		s.harvestUser(&e.User)
		s.harvestChannels(e.PrivateChannels)
		s.harvestGuildCreateEvents(e.Guilds)
		s.harvestRelationships(e.Relationships)
	case *gateway.ResumedEvent:
	case *gateway.InvalidSessionEvent:
	case *gateway.ChannelCreateEvent:
		s.harvestChannel(&e.Channel)
	case *gateway.ChannelUpdateEvent:
		s.harvestChannel(&e.Channel)
	case *gateway.ChannelDeleteEvent:
	case *gateway.ChannelPinsUpdateEvent:
	case *gateway.ChannelUnreadUpdateEvent:
	case *gateway.GuildCreateEvent:
		s.harvestGuildCreateEvent(e)
	case *gateway.GuildUpdateEvent:
		s.harvestGuild(&e.Guild)
	case *gateway.GuildDeleteEvent:
	case *gateway.GuildBanAddEvent:
	case *gateway.GuildBanRemoveEvent:
	case *gateway.GuildEmojisUpdateEvent:
	case *gateway.GuildIntegrationsUpdateEvent:
	case *gateway.GuildMemberAddEvent:
		s.harvestMember(e.GuildID, &e.Member)
	case *gateway.GuildMemberRemoveEvent:
	case *gateway.GuildMemberUpdateEvent:
		s.harvestUser(&e.User)
		s.harvestNick(e.GuildID, e.User.ID, e.Nick, e.User.Username)
	case *gateway.GuildMembersChunkEvent:
		s.harvestMembers(e.GuildID, e.Members)
	case *gateway.GuildMemberListUpdate:
	case *gateway.GuildRoleCreateEvent:
	case *gateway.GuildRoleUpdateEvent:
	case *gateway.GuildRoleDeleteEvent:
	case *gateway.InviteCreateEvent:
	case *gateway.InviteDeleteEvent:
	case *gateway.MessageCreateEvent:
		s.harvestMessage(&e.Message)
		s.harvestMember(e.GuildID, e.Member)
	case *gateway.MessageUpdateEvent:
		s.harvestMessage(&e.Message)
		s.harvestMember(e.GuildID, e.Member)
	case *gateway.MessageDeleteEvent:
	case *gateway.MessageDeleteBulkEvent:
	case *gateway.MessageReactionAddEvent:
		s.harvestMember(e.GuildID, e.Member)
	case *gateway.MessageReactionRemoveEvent:
	case *gateway.MessageReactionRemoveAllEvent:
	case *gateway.MessageAckEvent:
	case *gateway.PresenceUpdateEvent:
		s.harvestPresence(&e.Presence)
	case *gateway.PresencesReplaceEvent:
		s.harvestPresences(([]discord.Presence)(*e))
	case *gateway.SessionsReplaceEvent:
	case *gateway.TypingStartEvent:
		s.harvestMember(e.GuildID, e.Member)
	case *gateway.VoiceStateUpdateEvent:
		s.harvestMember(e.GuildID, e.Member)
	case *gateway.VoiceServerUpdateEvent:
	case *gateway.WebhooksUpdateEvent:
	case *gateway.UserSettingsUpdateEvent:
	case *gateway.UserGuildSettingsUpdateEvent:
	case *gateway.UserNoteUpdateEvent:
	case *gateway.RelationshipAddEvent:
		s.harvestUser(&e.User)
	case *gateway.RelationshipRemoveEvent:
		s.harvestUser(&e.User)
	}
}
