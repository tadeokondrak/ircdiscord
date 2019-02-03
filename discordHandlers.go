package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
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
		var ircChannel string
		var discordChannel *discordgo.Channel

		for _ircChannel, _discordChannel := range user.channels {
			if _discordChannel.ID == message.ChannelID {
				ircChannel = _ircChannel
				discordChannel = _discordChannel
				break
			}
		}

		if !user.joinedChannels[ircChannel] {
			continue
		}

		if discordChannel == nil {
			continue
		}

		if isRecentlySentMessage(user, message.Message) {
			continue
		}

		nick := getDiscordNick(user, message.Author)
		prefix := &irc.Prefix{
			Name: convertDiscordUsernameToIRCNick(nick),
			User: convertDiscordUsernameToIRCUser(message.Author.Username),
			Host: message.Author.ID,
		}

		// TODO: convert discord nicks to the irc nicks shown
		discordContent, err := message.ContentWithMoreMentionsReplaced(session)
		_ = err

		content := convertDiscordContentToIRC(discordContent, session)
		if content != "" {
			for _, line := range strings.Split(content, "\n") {
				user.Encode(&irc.Message{
					Prefix:  prefix,
					Command: irc.PRIVMSG,
					Params: []string{
						ircChannel,
						line,
					},
				})
			}
		}

		for _, attachment := range message.Attachments {
			user.Encode(&irc.Message{
				Prefix:  prefix,
				Command: irc.PRIVMSG,
				Params: []string{
					ircChannel,
					convertDiscordContentToIRC(attachment.URL, session),
				},
			})
		}
	}
}
