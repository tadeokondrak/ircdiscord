package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
)

// is there a better way?
func addHandlers(s *discordgo.Session) {
	s.AddHandler(messageCreate)
	s.AddHandler(messageDelete)
	s.AddHandler(messageUpdate)
	s.AddHandler(channelCreate)
	s.AddHandler(channelDelete)
	s.AddHandler(channelUpdate)
	s.AddHandler(guildRoleCreate)
	s.AddHandler(guildRoleDelete)
	s.AddHandler(guildRoleUpdate)
	s.AddHandler(guildMemberAdd)
	s.AddHandler(guildMemberRemove)
	s.AddHandler(guildMemberUpdate)
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	guildSession, exists := guildSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	guildSession.addMessage(message.Message)
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		date, err := message.Message.Timestamp.Parse()
		if err != nil {
			return
		}
		sendMessageFromDiscordToIRC(date, conn, message.Message, "", "")
	}
	guildSession.lastAck, _ = guildSession.session.ChannelMessageAck(message.ChannelID, message.ID, guildSession.lastAck.Token)
}

func messageUpdate(session *discordgo.Session, message *discordgo.MessageUpdate) {
	guildSession, exists := guildSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	oldMessage, err := guildSession.getMessage(message.ChannelID, message.ID)
	if err != nil {
		return
	}
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		date, err := message.Timestamp.Parse()
		if err != nil {
			return
		}
		sendMessageFromDiscordToIRC(date, conn, oldMessage, "\x0308message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f:\n", "")
		sendMessageFromDiscordToIRC(date, conn, message.Message, "\x0308was edited to:\n", "")
	}
	guildSession.addMessage(message.Message)
}

func messageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	guildSession, exists := guildSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, conn := range guildSession.conns {

		if conn == nil {
			continue
		}
		oldMessage, err := guildSession.getMessage(message.ChannelID, message.ID)
		if err != nil {
			return
		}
		sendMessageFromDiscordToIRC(time.Now(), conn, oldMessage, "\x0304message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f \x0304in this channel was deleted:\n", "")
		return
	}
}

func channelCreate(session *discordgo.Session, channel *discordgo.ChannelCreate) {
	guildSession, exists := guildSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	guildSession.addChannel(channel.Channel)
}

func channelDelete(session *discordgo.Session, channel *discordgo.ChannelDelete) {
	// TODO: kick user from channel
	guildSession, exists := guildSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	guildSession.removeChannel(channel.Channel)
}

func channelUpdate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	// TODO: handle channel name changes somehow
	guildSession, exists := guildSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	guildSession.updateChannel(channel.Channel)
}

func guildRoleCreate(session *discordgo.Session, role *discordgo.GuildRoleCreate) {
	guildSession, exists := guildSessions[session.Token][role.GuildID]
	if !exists {
		return
	}
	guildSession.addRole(role.Role)
}

func guildRoleDelete(session *discordgo.Session, role *discordgo.GuildRoleDelete) {
	guildSession, exists := guildSessions[session.Token][role.GuildID]
	if !exists {
		return
	}
	guildSession.removeRole(role.RoleID)
}

func guildRoleUpdate(session *discordgo.Session, role *discordgo.GuildRoleUpdate) {
	// TODO: handle channel name changes somehow
	guildSession, exists := guildSessions[session.Token][role.GuildID]
	if !exists {
		return
	}
	guildSession.updateRole(role.Role)
}

func guildMemberAdd(session *discordgo.Session, member *discordgo.GuildMemberAdd) {
	guildSession, exists := guildSessions[session.Token][member.GuildID]
	if !exists {
		return
	}
	guildSession.addMember(member.Member)
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		for ircChannel := range conn.channels {
			conn.sendJOIN(
				conn.getNick(member.User),
				convertDiscordUsernameToIRCRealname(member.Member.User.Username),
				member.Member.User.ID,
				ircChannel,
			)
		}
		return
	}
}

func guildMemberUpdate(session *discordgo.Session, member *discordgo.GuildMemberUpdate) {
	guildSession, exists := guildSessions[session.Token][member.GuildID]
	if !exists {
		return
	}
	guildSession.updateMember(member.Member)
	// TODO: handle nick changes? handle role changes?
}

func guildMemberRemove(session *discordgo.Session, member *discordgo.GuildMemberRemove) {
	guildSession, exists := guildSessions[session.Token][member.GuildID]
	if !exists {
		return
	}
	guildSession.removeMember(member.Member)
	// TODO: send part like guildMemberAdd
}
