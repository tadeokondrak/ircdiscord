package main

import (
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func reloadChannels(session *discordgo.Session, guildID string) {
	userSlice, ok := ircSessions[session.Token][guildID]
	if !ok {
		return
	}
	for _, user := range userSlice {
		newChannels := map[string]*discordgo.Channel{} // clear user.channels
		channels, _ := user.session.GuildChannels(user.guildID)
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildCategory || channel.Type == discordgo.ChannelTypeGuildVoice {
				continue
			}
			name := convertDiscordChannelNameToIRC(channel.Name)
			done := false
			var suffix string
			for i := 0; !done; i++ {
				if i == 0 {
					suffix = ""
				} else {
					suffix = strconv.Itoa(i)
				}
				_, ok := newChannels[name+suffix]
				if !ok {
					newChannels[name+suffix] = channel
					done = true
				}
			}
			user.channels = newChannels
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
