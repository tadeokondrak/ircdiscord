package main

import (
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func convertDiscordChannelNameToIRC(discordName string) (IRCName string) {
	re := regexp.MustCompile(`[^a-zA-Z0-9\-_#&]+`)
	cleaned := re.ReplaceAllString(discordName, "")
	IRCName = truncate("#"+cleaned, 50)

	if IRCName == "" {
		IRCName = "_"
	}

	return
}

func removeWhitespace(input string) string {
	re := regexp.MustCompile("[\x20\x00\x0d\x0a]+")
	return re.ReplaceAllString(input, "")
}

func removeWhitespaceAndComma(input string) string {
	re := regexp.MustCompile("[,]+")
	return re.ReplaceAllString(removeWhitespace(input), "")
}

func underscoreIfEmpty(input string) string {
	if input == "" {
		return "_"
	}
	return input
}

func convertDiscordUsernameToIRCUser(name string) string {
	return underscoreIfEmpty(removeWhitespace(name))
}

func convertDiscordUsernameToIRCRealname(name string) string {
	return underscoreIfEmpty(removeWhitespaceAndComma(name))
}

func getIRCNick(name string) (nick string) {
	// if user != nil {
	// if member.Nick != "" {
	// name = member.Nick
	// } else {
	// name = member.User.Username
	// }
	// }

	re := regexp.MustCompile(`[^A-Za-z0-9\-[\]\\\x60\{\}]+`)
	nick = underscoreIfEmpty(re.ReplaceAllString(name, ""))

	// // must not start with number
	// r, _ := utf8.DecodeRuneInString(nick)
	// if r > '0' && r < '9' {
	// 	return "_" + nick
	// }
	return nick
}

func convertDiscordTopicToIRC(discordContent string, session *discordgo.Session) (ircContent string) {
	content := convertDiscordContentToIRC(discordContent)
	newlines := regexp.MustCompile("[\n]+")
	ircContent = newlines.ReplaceAllString(content, "")
	return
}

func convertDiscordContentToIRC(discordContent string) (ircContent string) {
	return discordContent
}

func replaceMentions(c *ircConn, m *discordgo.Message) (content string) {
	content = m.Content
	for _, mentionedUser := range m.Mentions {
		username := c.guildSession.userMap.GetName(mentionedUser.ID)
		colour := "12"
		if mentionedUser.ID == c.selfUser.ID {
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
		for _, role := range c.guildSession.selfMember.Roles {
			if roleID == role {
				colour = "03\x16"
			}
		}
		content = strings.Replace(content, "<@&"+roleID+">", "\x03"+colour+"\x02@"+roleName+"\x0f", -1)
	}
	return
}
