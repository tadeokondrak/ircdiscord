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
		sendMessageFromDiscordToIRC(conn, message.Message, "")
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
		sendMessageFromDiscordToIRC(conn, message.Message, "\x0308message sent \x0f\x02"+humanize.Time(getTimeFromSnowflake(message.ID))+"\x0f \x0308edited to:\n")
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
		conn.sendPRIVMSG(time.Now(), "", "", "",
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
