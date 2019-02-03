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

func isRecentlySentMessage(user *ircUser, message *discordgo.Message) bool {
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

func sendMessageFromDiscordToIRC(user *ircUser, message *discordgo.Message) {
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
		return
	}

	if discordChannel == nil {
		return
	}

	if isRecentlySentMessage(user, message) {
		return
	}

	nick := getDiscordNick(user, message.Author)
	prefix := &irc.Prefix{
		Name: convertDiscordUsernameToIRCNick(nick),
		User: convertDiscordUsernameToIRCUser(message.Author.Username),
		Host: message.Author.ID,
	}

	// TODO: convert discord nicks to the irc nicks shown
	discordContent, err := message.ContentWithMoreMentionsReplaced(user.session)
	_ = err

	content := convertDiscordContentToIRC(discordContent, user.session)
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
	if discordChannel.Type == discordgo.ChannelTypeGuildCategory || discordChannel.Type == discordgo.ChannelTypeGuildVoice {
		return
	}
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
			addChannel(user, channel)
		}
	}
}

func getDiscordNick(user *ircUser, discordUser *discordgo.User) (nick string) {
	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return discordUser.Username
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

	if member.Nick != "" && len(discordUser.Username) > len(member.Nick) {
		return member.Nick
	}
	return discordUser.Username
}

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(ircMessage string) (discordMessage string) {
	discordMessage = strings.TrimSpace(ircMessage)
	return ircMessage
}
