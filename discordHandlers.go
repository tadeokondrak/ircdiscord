package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/dustin/go-humanize"
	"github.com/tadeokondrak/irc"
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
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	guildSession, exists := guildSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		sendMessageFromDiscordToIRC(conn, message.Message, "", "")
	}
}

func messageUpdate(session *discordgo.Session, message *discordgo.MessageUpdate) {
	guildSession, exists := guildSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, conn := range guildSession.conns {
		if conn == nil {
			continue
		}
		sendMessageFromDiscordToIRC(conn, message.Message, "\x0308message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f \x0308edited to:\n", "")
	}
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
		tags := irc.Tags{
			"time": time.Now().Format("2006-01-02T15:04:05.000Z"),
		}
		conn.sendPRIVMSG(tags, "", "", "",
			conn.guildSession.channelMap.GetName(message.ChannelID),
			"\x0304message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f \x0304in this channel was deleted",
		)
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
