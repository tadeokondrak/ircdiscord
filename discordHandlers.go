package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func addRecentlySentMessage(user *ircUser, channelID string, content string) {
	if user.recentlySentMessages == nil {
		user.recentlySentMessages = map[string][]string{}
	}
	user.recentlySentMessages[channelID] = append(user.recentlySentMessages[channelID], content)
}

func isRecentlySentMessage(user *ircUser, channelID string, content string) bool {
	// TODO: verify that the message was sent by us
	if recentlySentMessages, ok := user.recentlySentMessages[channelID]; ok {
		for index, recentMessage := range recentlySentMessages {
			if content == recentMessage && recentMessage != "" {
				user.recentlySentMessages[channelID][index] = "" // remove the message from recently sent
				return true
			}
		}
	}
	return false
}

func channelCreate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	reloadChannels(session, channel.GuildID)
}

func channelDelete(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	reloadChannels(session, channel.GuildID)
}

func channelUpdate(session *discordgo.Session, channel *discordgo.ChannelUpdate) {
	reloadChannels(session, channel.GuildID)
}

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	userSlice, ok := ircSessions[session.Token][message.GuildID]
	if !ok {
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

		if isRecentlySentMessage(user, message.ChannelID, message.Content) {
			continue
		}

		nick := getDiscordNick(user, message.Author)
		prefix := &irc.Prefix{
			Name: convertDiscordUsernameToIRC(nick),
			User: convertDiscordUsernameToIRC(message.Author.Username),
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
