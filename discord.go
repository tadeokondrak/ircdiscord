package main

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func getDiscordNick(user *ircUser, discordUser *discordgo.User) (nick string) {
	nick = discordUser.Username

	if discordUser.Discriminator == "0000" {
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
