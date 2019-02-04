package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func getTimeFromSnowflake(snowflake string) time.Time {
	var snowInt, unix uint64
	snowInt, _ = strconv.ParseUint(snowflake, 10, 64)
	fmt.Println(snowInt)
	unix = (snowInt >> 22) + 1420070400000
	fmt.Println(unix)
	return time.Unix(0, int64(unix)*1000000)
}
func addRecentlySentMessage(user *ircUser, channelID string, content string) {
	if user.recentlySentMessages == nil {
		user.recentlySentMessages = map[string][]string{}
	}
	user.recentlySentMessages[channelID] = append(user.recentlySentMessages[channelID], content)
}

func isRecentlySentMessage(user *ircUser, message *discordgo.Message) bool {
	if message == nil || user.discordUser == nil {
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

func sendMessageFromDiscordToIRC(user *ircUser, message *discordgo.Message, prefixString string) {
	ircChannel := user.channels.getFromSnowflake(message.ChannelID)

	if ircChannel == "" {
		return
	}

	if !user.joinedChannels[ircChannel] {
		return
	}

	if isRecentlySentMessage(user, message) {
		return
	}

	nick := user.users.getNick(message.Author)
	prefix := &irc.Prefix{
		Name: nick,
		User: nick,
		Host: message.Author.ID,
	}

	// TODO: convert discord nicks to the irc nicks shown
	discordContent, err := message.ContentWithMoreMentionsReplaced(user.session)
	_ = err

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

func convertIRCMessageToDiscord(ircMessage string) (discordMessage string) {
	discordMessage = strings.TrimSpace(ircMessage)
	return ircMessage
}
