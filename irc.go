package main

import (
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func convertDiscordChannelNameToIRC(discordName string) (IRCName string) {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_]+`)
	cleaned := re.ReplaceAllString(discordName, "")
	return truncate("#"+cleaned, 50)
}

func convertDiscordUsernameToIRCUser(discordName string) (IRCUser string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\]+") //T ODO: `
	cleaned := re.ReplaceAllString(discordName, "")

	IRCUser = truncate(cleaned, 20) // arbitrary limit, i couldn't find a real one

	if IRCUser == "" {
		IRCUser = "_"
	}

	return
}

func convertDiscordUsernameToIRCRealname(discordName string) (IRCName string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\ ]+") // TODO: `
	cleaned := re.ReplaceAllString(discordName, "")

	IRCName = truncate(cleaned, 20) // arbitrary limit, i couldn't find a real one

	if IRCName == "" {
		IRCName = "_"
	}

	return
}

func convertDiscordUsernameToIRCNick(discordName string) (IRCNick string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|\\\\]+") // TODO: `
	cleaned := re.ReplaceAllString(discordName, "")

	IRCNick = truncate(cleaned, 12)

	if IRCNick == "" {
		IRCNick = "_"
	}

	return
}

func convertDiscordTopicToIRC(discordContent string, session *discordgo.Session) (ircContent string) {
	content := convertDiscordContentToIRC(discordContent, session)
	newlines := regexp.MustCompile("[\n]+")
	ircContent = newlines.ReplaceAllString(content, "")
	return
}

func convertDiscordContentToIRC(discordContent string, session *discordgo.Session) (ircContent string) {
	return discordContent
}

func replaceMentions(c *ircConn, m *discordgo.Message) (content string) {
	content = m.Content
	for _, mentionedUser := range m.Mentions {
		username := c.guildSession.userMap.GetName(mentionedUser.ID)
		colour := "12"
		if mentionedUser.ID == c.self.User.ID {
			colour = "12\x16"
		}
		if username != "" {
			content = strings.NewReplacer(
				"<@"+mentionedUser.ID+">", "\x03"+colour+"\x02@"+username+"\x0f",
				"<@!"+mentionedUser.ID+">", "\x03"+colour+"\x02@"+username+"\x0f",
			).Replace(content)
		}
	}
	for _, roleID := range m.MentionRoles {
		roleName := c.guildSession.roleMap.GetName(roleID)
		colour := "03"
		for _, role := range c.guildSession.self.Roles {
			if roleID == role {
				colour = "03\x16"
			}
		}
		content = strings.Replace(content, "<@&"+roleID+">", "\x03"+colour+"\x02@"+roleName+"\x0f", -1)
	}
	return
}
