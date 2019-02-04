package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func replaceMentions(user *ircUser, message *discordgo.Message) (content string) {
	content = message.Content
	for _, mentionedUser := range message.Mentions {
		username := user.users.getFromSnowflake(mentionedUser.ID)
		if username != "" {
			content = strings.NewReplacer(
				"<@"+mentionedUser.ID+">", "\x0312\x02@"+username+"\x0f",
				"<@!"+mentionedUser.ID+">", "\x0312\x02@"+username+"\x0f",
			).Replace(content)
		}
	}
	return
}

func getTimeFromSnowflake(snowflake string) time.Time {
	var snowInt, unix uint64
	snowInt, _ = strconv.ParseUint(snowflake, 10, 64)
	unix = (snowInt >> 22) + 1420070400000
	return time.Unix(0, int64(unix)*1000000)
}
func addRecentlySentMessage(user *ircUser, channelID string, content string) {
	if user.recentlySentMessages == nil {
		user.recentlySentMessages = map[string][]string{}
	}
	user.recentlySentMessages[channelID] = append(user.recentlySentMessages[channelID], content)
}

func isRecentlySentMessage(user *ircUser, message *discordgo.Message) bool {
	if message.Author == nil || user.discordUser == nil {
		return false
	}
	if user.discordUser.ID != message.Author.ID {
		return false
	}
	if recentlySentMessages, exists := user.recentlySentMessages[message.ChannelID]; exists {
		for index, recentMessage := range recentlySentMessages {
			if message.Content == recentMessage && recentMessage != "" {
				user.recentlySentMessages[message.ChannelID][index] = "" // remove the message from recently sent
				return true
			}
		}
	}
	return false
}

func convertIRCMentionsToDiscord(user *ircUser, message string) (content string) {
	// TODO: allow chained mentions (`user1: user2: `)
	startMessageMentionRegex := regexp.MustCompile(`^([^:]+):\ `)
	matches := startMessageMentionRegex.FindAllStringSubmatchIndex(message, -1)
	if len(matches) == 0 {
		return message
	}
	discordID := user.users.get(message[matches[0][2]:matches[0][3]])
	if discordID != "" {
		return "<@" + discordID + "> " + message[matches[0][1]:]
	}
	return message
}

func sendMessageFromDiscordToIRC(user *ircUser, message *discordgo.Message, prefixString string) {
	ircChannel := user.channels.getFromSnowflake(message.ChannelID)

	if ircChannel == "" || !user.joinedChannels[ircChannel] || isRecentlySentMessage(user, message) || message.Author == nil {
		return
	}

	nick := user.users.getNick(message.Author)
	prefix := &irc.Prefix{
		Name: nick,
		User: nick,
		Host: message.Author.ID,
	}

	// TODO: check if edited and put (edited) with low contrast
	discordContent := replaceMentions(user, message)

	content := prefixString + convertDiscordContentToIRC(discordContent, user.session)
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
				convertDiscordContentToIRC(attachment.URL, user.session),
			},
		})
	}
}

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(user *ircUser, ircMessage string) (discordMessage string) {
	discordMessage = ircMessage
	discordMessage = strings.TrimSpace(discordMessage)
	discordMessage = convertIRCMentionsToDiscord(user, discordMessage)
	return discordMessage
}
