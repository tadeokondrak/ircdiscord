package main

import (
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

func messageCreate(session *discordgo.Session, message *discordgo.MessageCreate) {
	userSlice, ok := ircSessions[session.Token][message.GuildID]
	if !ok {
		return
	}
	for _, user := range userSlice {
		channel, ok := user.channels[message.ChannelID]

		if !ok {
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

		content := convertDiscordContentToIRC(discordContent, session) // SPLIT BY LINES AFTER
		if content != "" {
			user.Encode(&irc.Message{
				Prefix:  prefix,
				Command: irc.PRIVMSG,
				Params: []string{
					channel,
					content,
				},
			})
		}

		for _, attachment := range message.Attachments {
			user.Encode(&irc.Message{
				Prefix:  prefix,
				Command: irc.PRIVMSG,
				Params: []string{
					channel,
					convertDiscordContentToIRC(attachment.URL, session),
				},
			})
		}
	}
}
