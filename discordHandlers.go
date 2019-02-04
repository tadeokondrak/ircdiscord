package main

import (
	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func messageUpdate(session *discordgo.Session, message *discordgo.MessageUpdate) {
	userSlice, exists := ircSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		sendMessageFromDiscordToIRC(user, message.Message, "\x02\x0308edited to:\x0f ")
	}
}

func messageDelete(session *discordgo.Session, message *discordgo.MessageDelete) {
	userSlice, exists := ircSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		user.Encode(&irc.Message{
			Prefix:  &irc.Prefix{},
			Command: irc.PRIVMSG,
			Params: []string{
				user.channels.getFromSnowflake(message.ChannelID),
				"\x0304a message in this channel was deleted",
			},
		})
		return
	}
}

func channelCreate(session *discordgo.Session, channel *discordgo.ChannelCreate) {
	userSlice, exists := ircSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		user.channels.addChannel(channel.Channel)
	}
}

func channelDelete(session *discordgo.Session, channel *discordgo.ChannelDelete) {
	userSlice, exists := ircSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		user.channels.removeFromSnowflake(channel.Channel.ID)
	}
}

func channelUpdate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	// TODO: handle channel name changes somehow
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	userSlice, exists := ircSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		sendMessageFromDiscordToIRC(user, message.Message, "")
	}
}
