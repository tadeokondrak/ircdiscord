package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/tadeokondrak/irc"
)

func getTimeFromSnowflake(snowflake string) time.Time {
	var snowInt, unix uint64
	snowInt, _ = strconv.ParseUint(snowflake, 10, 64)
	unix = (snowInt >> 22) + 1420070400000
	return time.Unix(0, int64(unix)*1000000)
}
func addRecentlySentMessage(user *ircConn, channelID string, content string) {
	user.recentlySentMessages[channelID] = append(user.recentlySentMessages[channelID], content)
}

func isRecentlySentMessage(user *ircConn, message *discordgo.Message) bool {
	if message.Author == nil || user.guildSession.self == nil {
		return false
	}
	if user.guildSession.self.User.ID != message.Author.ID {
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

func convertIRCMentionsToDiscord(user *ircConn, message string) (content string) {
	// TODO: allow chained mentions (`user1: user2: `)
	startMessageMentionRegex := regexp.MustCompile(`^([^:]+):\ `)
	matches := startMessageMentionRegex.FindAllStringSubmatchIndex(message, -1)
	if len(matches) == 0 {
		return message
	}
	discordID := user.guildSession.userMap.GetSnowflake(message[matches[0][2]:matches[0][3]])
	if discordID != "" {
		return "<@" + discordID + "> " + message[matches[0][1]:]
	}
	return message
}

func (g *guildSession) sendMessageFromDiscordToIRC(message *discordgo.Message) {}

func sendMessageFromDiscordToIRC(date time.Time, user *ircConn, message *discordgo.Message, prefixString string, batchTag string) {
	ircChannel := user.guildSession.channelMap.GetName(message.ChannelID)

	if ircChannel == "" || !user.channels[ircChannel] || isRecentlySentMessage(user, message) || message.Author == nil {
		return
	}

	tags := irc.Tags{
		"time": date.Format("2006-01-02T15:04:05.000Z"),
	}

	if batchTag != "" {
		tags["batch"] = batchTag
	}

	nick := user.guildSession.getNick(message.Author)

	// TODO: check if edited and put (edited) with low contrast
	discordContent := replaceMentions(user, message)

	content := prefixString + convertDiscordContentToIRC(discordContent, user.session)
	if content != "" {
		for _, line := range strings.Split(content, "\n") {
			user.sendPRIVMSG(tags, nick, nick, message.Author.ID, ircChannel, line)
		}
	}

	for _, attachment := range message.Attachments {
		user.sendPRIVMSG(tags, nick, nick, message.Author.ID, ircChannel, convertDiscordContentToIRC(attachment.URL, user.session))
	}
}

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(user *ircConn, ircMessage string) (discordMessage string) {
	discordMessage = ircMessage
	discordMessage = strings.TrimSpace(discordMessage)
	discordMessage = convertIRCMentionsToDiscord(user, discordMessage)
	return discordMessage
}
