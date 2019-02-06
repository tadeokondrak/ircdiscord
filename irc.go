package main

import (
	"regexp"

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
