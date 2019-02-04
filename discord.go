package main

import (
	"strconv"
	"strings"
	"sync"

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

type snowflakeMap struct {
	table map[string]string
	sync.Mutex
}

func newSnowflakeMap() *snowflakeMap {
	return &snowflakeMap{
		table: map[string]string{},
	}
}

func (m *snowflakeMap) add(name string, snowflake string, separator string) string {
	m.Lock()
	defer m.Unlock()
	var suffix string
	for i := 0; ; i++ {
		if i == 0 {
			suffix = ""
		} else {
			suffix = separator + strconv.Itoa(i)
		}
		_name := name + suffix
		_, exists := m.table[_name]
		if !exists {
			m.table[_name] = snowflake
			return _name
		}
	}
}

func (m *snowflakeMap) get(name string) string {
	snowflake, exists := m.table[name]
	if exists {
		return snowflake
	}
	return ""
}

func (m *snowflakeMap) getMap() map[string]string {
	return m.table
}

func (m *snowflakeMap) getFromSnowflake(snowflake string) string {
	for name, _snowflake := range m.table {
		if _snowflake == snowflake {
			return name
		}
	}
	return ""
}

func (m *snowflakeMap) removeFromSnowflake(snowflake string) string {
	name := m.getFromSnowflake(snowflake)
	m.remove(name)
	return name
}

func (m *snowflakeMap) remove(name string) {
	delete(m.table, name)
}

func (m *snowflakeMap) clear() {
	m = newSnowflakeMap()
}

func (m *snowflakeMap) addChannel(channel *discordgo.Channel) string {
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeDM {
		return ""
	}
	return m.add(convertDiscordChannelNameToIRC(channel.Name), channel.ID, "#")
}

func (m *snowflakeMap) addUser(user *discordgo.User) string {
	if user.Discriminator == "0000" {
		// We don't add users for webhooks because they're not users
		return ""
	}
	return m.add(convertDiscordUsernameToIRCNick(user.Username), user.ID, "@")
}

func (m *snowflakeMap) getNick(discordUser *discordgo.User) string {
	username := convertDiscordUsernameToIRCNick(discordUser.Username)
	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return username + "@w"
	}
	return username
}

// func getDiscordNick(user *ircUser, discordUser *discordgo.User) (nick string) {
// 	member, err := user.session.State.Member(user.guildID, discordUser.ID)
// 	if err != nil {
// 		member, err = user.session.GuildMember(user.guildID, discordUser.ID)
// 		if err != nil {
// 			user.Encode(&irc.Message{
// 				Prefix:  user.serverPrefix,
// 				Command: irc.NOTICE,
// 				Params:  []string{user.nick, "There was an error getting member data from Discord."},
// 			})
// 			return
// 		}
// 	}
//
// 	if member.Nick != "" && len(discordUser.Username) > len(member.Nick) {
// 		return member.Nick
// 	}
// 	return discordUser.Username
// }

func isValidDiscordNick(nick string) bool {
	return true
}

func convertIRCMessageToDiscord(ircMessage string) (discordMessage string) {
	discordMessage = strings.TrimSpace(ircMessage)
	return ircMessage
}
