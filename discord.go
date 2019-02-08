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
	if message.Author == nil || user.guildSession.selfUser == nil {
		return false
	}
	if user.guildSession.selfUser.ID != message.Author.ID {
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

func sendMessageFromDiscordToIRC(date time.Time, c *ircConn, m *discordgo.Message, prefixString string, batchTag string) {
	ircChannel := c.guildSession.channelMap.GetName(m.ChannelID)
	c.channelsMutex.Lock()
	if c.guildSession.guildSessionType == guildSessionGuild && !c.channels[m.ChannelID] {
		c.channelsMutex.Unlock()
		return
	}
	c.channelsMutex.Unlock()

	if ircChannel == "" || isRecentlySentMessage(c, m) || m.Author == nil {
		return
	}

	tags := irc.Tags{
		"time": date.Format("2006-01-02T15:04:05.000Z"),
	}

	if batchTag != "" {
		tags["batch"] = batchTag
	}

	nick := c.guildSession.getNick(m.Author)

	// TODO: check if edited and put (edited) with low contrast
	discordContent := replaceMentions(c, m)

	content := prefixString + convertDiscordContentToIRC(discordContent)
	if content != "" {
		for _, line := range strings.Split(content, "\n") {
			c.sendPRIVMSG(tags, nick, nick, m.Author.ID, ircChannel, line)
		}
	}

	for _, attachment := range m.Attachments {
		c.sendPRIVMSG(tags, nick, nick, m.Author.ID, ircChannel, convertDiscordContentToIRC(attachment.URL))
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
