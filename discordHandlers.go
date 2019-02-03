package main

import (
	"github.com/bwmarrin/discordgo"
)

func channelCreate(session *discordgo.Session, channel *discordgo.ChannelCreate) {
	userSlice, exists := ircSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		addChannel(user, channel.Channel)
	}
}

func channelDelete(session *discordgo.Session, channel *discordgo.ChannelDelete) {
	userSlice, exists := ircSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		removeChannel(user, channel.Channel)
	}
}

func channelUpdate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	userSlice, exists := ircSessions[session.Token][channel.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		updateChannel(user, channel.Channel)
	}
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	userSlice, exists := ircSessions[session.Token][message.GuildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		sendMessageFromDiscordToIRC(user, message.Message)
	}
}
