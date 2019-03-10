package main

import (
	"fmt"
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
	re := regexp.MustCompile(`[^A-Za-z0-9\-[\]\\\x60\{\}]+`)
	nick = underscoreIfEmpty(re.ReplaceAllString(name, ""))

	return nick
}

func convertDiscordTopicToIRC(discordContent string, c *ircConn) (ircContent string) {
	content := convertDiscordContentToIRC(discordContent, c)
	newlines := regexp.MustCompile("[\n]+")
	ircContent = newlines.ReplaceAllString(content, "")
	return
}

var (
	patternChannels = regexp.MustCompile("<#[^>]*>")
	patternUsers    = regexp.MustCompile("<@[^>]*>")
	patternNicks    = regexp.MustCompile("<@![^>]*>")
	patternRoles    = regexp.MustCompile("<@&[^>]*>")
	patternEmoji    = regexp.MustCompile("<a?:[^>]*:[^>]*>")
	//patternBold          = regexp.MustCompile(`\*{2}([^*].*)\*{2}`)
	//patternItalic1       = regexp.MustCompile(`\*([^*].*)\*`)
	//patternUnderline     = regexp.MustCompile(`_{2}([^_].*)_{2}`)
	//patternItalic2       = regexp.MustCompile(`_([^_].*)_`)
	//patternSpoiler       = regexp.MustCompile(`\|{2}([^|].*)\|{2}`)
	//patternSmallCode     = regexp.MustCompile(`\x60([^\x60]*)\x60`)
	//patternStrikethrough = regexp.MustCompile(`~{2}([^~].*)~{2}`)
)

func convertDiscordContentToIRC(text string, c *ircConn) (content string) {
	content = text
	content = patternRoles.ReplaceAllStringFunc(content, func(mention string) string {
		role, err := c.getRole(mention[3 : len(mention)-1])
		if err != nil {
			return mention
		}

		colour := "\x0303\x02"
		colourReset := "\x03\x02"
		for _, _role := range c.guildSession.selfMember.Roles {
			if role.ID == _role {
				colour += "\x16"
				colourReset += "\x16"
			}
		}

		return fmt.Sprintf("%s&%s%s", colour, c.getRoleName(role), colourReset)
	})

	// TODO: remove this copy/paste shit
	content = patternNicks.ReplaceAllStringFunc(content, func(mention string) string {
		user, err := c.getUser(mention[3 : len(mention)-1])
		if err != nil {
			return mention
		}

		colour := "\x0302\x02"
		colourReset := "\x03\x02"
		if user.ID == c.selfUser.ID {
			colour += "\x16"
			colourReset += "\x16"
		}

		return fmt.Sprintf("%s@%s%s", colour, c.getNick(user), colourReset)
	})

	content = patternUsers.ReplaceAllStringFunc(content, func(mention string) string {
		user, err := c.getUser(mention[2 : len(mention)-1])
		if err != nil {
			return mention
		}

		colour := "\x0302\x02"
		colourReset := "\x03\x02"
		if user.ID == c.selfUser.ID {
			colour += "\x16"
			colourReset += "\x16"
		}

		return fmt.Sprintf("%s@%s%s", colour, c.getNick(user), colourReset)
	})

	content = patternChannels.ReplaceAllStringFunc(content, func(mention string) string {
		channel, err := c.getChannel(mention[2 : len(mention)-1])
		if err != nil {
			return mention
		}
		// TODO: remove # from channel name and add it back here
		return fmt.Sprintf("\x0304\x02%s\x03\x02", c.getChannelName(channel))
	})

	content = patternEmoji.ReplaceAllStringFunc(content, func(match string) string {
		return fmt.Sprintf("\x0305:%s:\x03", strings.Split(match[2:len(match)-1], ":")[0])
	})

	// content = patternBold.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x02%s\x02", match[2:len(match)-2])
	// })

	// content = patternItalic1.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x1d%s\x1d", match[1:len(match)-1])
	// })

	// content = patternUnderline.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x1f%s\x1f", match[2:len(match)-2])
	// })

	// content = patternItalic2.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x1d%s\x1d", match[1:len(match)-1])
	// })

	// content = patternSpoiler.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x0301,01%s\x03", match[2:len(match)-2])
	// })

	// content = patternSmallCode.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x11%s\x11", match[1:len(match)-1])
	// })

	// content = patternStrikethrough.ReplaceAllStringFunc(content, func(match string) string {
	// 	return fmt.Sprintf("\x1e%s\x1e", match[2:len(match)-2])
	// })

	return
}

func convertDiscordMessageToIRC(m *discordgo.Message, c *ircConn) (ircContent string) {
	// TODO: check if edited and put (edited) with low contrast
	ircContent = m.Content
	for _, attachment := range m.Attachments {
		if ircContent != "" {
			ircContent += "\n"
		}
		ircContent += attachment.URL
	}
	ircContent = convertDiscordContentToIRC(ircContent, c)
	return
}
