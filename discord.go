package main

import (
	"strconv"
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
	if recentlySentMessages, exists := user.recentlySentMessages[channelID]; exists {
		for index, recentMessage := range recentlySentMessages {
			if content == recentMessage && recentMessage != "" {
				user.recentlySentMessages[channelID][index] = "" // remove the message from recently sent
				return true
			}
		}
	}
	return false
}

func getNameForChannel(user *ircUser, discordChannel *discordgo.Channel) (name string, exists bool) {
	channelName := convertDiscordChannelNameToIRC(discordChannel.Name)
	var suffix string
	for i := 0; ; i++ {
		if i == 0 {
			suffix = ""
		} else {
			suffix = strconv.Itoa(i)
		}
		name := channelName + suffix
		_discordChannel, exists := user.channels[name]
		if !exists {
			return name, false
		}
		if _discordChannel.ID == discordChannel.ID {
			return name, true
		}
	}
}

// returns empty string if it can't find
func getChannelByID(user *ircUser, channelID string) (name string, channel *discordgo.Channel) {
	for ircChannel, discordChannel := range user.channels {
		if discordChannel.ID == channelID {
			return ircChannel, discordChannel
		}
	}
	return "", nil
}

func addChannel(user *ircUser, discordChannel *discordgo.Channel) {
	ircChannel, exists := getNameForChannel(user, discordChannel)
	if exists {
		return
	}
	user.channels[ircChannel] = discordChannel
}

func removeChannel(user *ircUser, discordChannel *discordgo.Channel) {
	// TODO: kick the user if they're in it
	ircChannel, _ := getChannelByID(user, discordChannel.ID)
	if ircChannel == "" {
		return
	}
	delete(user.channels, ircChannel)
}

func updateChannel(user *ircUser, discordChannel *discordgo.Channel) {
	// TODO: handle the channel name changing better, dunno how though
	ircChannel, _ := getChannelByID(user, discordChannel.ID)
	if ircChannel == "" {
		return
	}
	user.channels[ircChannel] = discordChannel
}

func loadChannels(session *discordgo.Session, guildID string) {
	userSlice, exists := ircSessions[session.Token][guildID]
	if !exists {
		return
	}
	for _, user := range userSlice {
		user.channels = map[string]*discordgo.Channel{} // clear user.channels
		channels, _ := user.session.GuildChannels(user.guildID)
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildCategory || channel.Type == discordgo.ChannelTypeGuildVoice {
				continue
			}
			addChannel(user, channel)
		}
	}
}

func getDiscordNick(user *ircUser, discordUser *discordgo.User) (nick string) {
	nick = discordUser.Username

	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return
	}

	member, err := user.session.State.Member(user.guildID, discordUser.ID)
	if err != nil {
		member, err = user.session.GuildMember(user.guildID, discordUser.ID)
		if err != nil {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.NOTICE,
				Params:  []string{user.nick, "There was an error getting member data from Discord."},
			})
			return
		}
	}

	if member.Nick != "" {
		nick = member.Nick
	}

	return
}

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(ircMessage string) (discordMessage string) {
	discordMessage = strings.TrimSpace(ircMessage)
	return ircMessage
}
