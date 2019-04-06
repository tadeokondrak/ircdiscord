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

func addRecentlySentMessage(c *ircConn, channelID string, content string) {
	c.recentlySentMessages[channelID] = append(c.recentlySentMessages[channelID], content)
}

func isRecentlySentMessage(c *ircConn, m *discordgo.Message) bool {
	if c.user.supportedCapabilities["echo-message"] {
		return false
	}
	if m.Author == nil || c.guildSession.selfUser == nil {
		return false
	}
	if c.guildSession.selfUser.ID != m.Author.ID {
		return false
	}
	if recentlySentMessages, exists := c.recentlySentMessages[m.ChannelID]; exists {
		for index, recentMessage := range recentlySentMessages {
			if m.Content == recentMessage && recentMessage != "" {
				c.recentlySentMessages[m.ChannelID][index] = "" // remove the message from recently sent
				return true
			}
		}
	}
	return false
}

func convertIRCMentionsToDiscord(c *ircConn, message string) (content string) {
	// TODO: allow chained mentions (`user1: user2: `)
	startMessageMentionRegex := regexp.MustCompile(`^([^:]+):`)
	matches := startMessageMentionRegex.FindAllStringSubmatchIndex(message, -1)
	if len(matches) == 0 {
		return message
	}
	discordID := c.guildSession.userMap.GetSnowflake(message[matches[0][2]:matches[0][3]])
	if discordID != "" {
		return "<@" + discordID + "> " + message[matches[0][1]:]
	}
	return message
}

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

	content := prefixString + convertDiscordMessageToIRC(m, c)
	if content != "" {
		for _, line := range strings.Split(content, "\n") {
			c.sendPRIVMSG(tags, nick, nick, m.Author.ID, ircChannel, line)
		}
	}
}

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(user *ircConn, ircMessage string) (discordMessage string) {
	actionRegex := regexp.MustCompile(`^\x01ACTION (.*)\x01$`)
	discordMessage = ircMessage
	discordMessage = strings.TrimSpace(discordMessage)
	discordMessage = actionRegex.ReplaceAllString(discordMessage, `*$1*`)
	discordMessage = convertIRCMentionsToDiscord(user, discordMessage)
	return discordMessage
}
