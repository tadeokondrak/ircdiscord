package main

import (
	"bufio"
	"fmt"
	"net"
	"regexp"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

type ircUser struct {
	nick                 string
	realName             string
	hostname             string
	discordUser          *discordgo.User
	channels             *snowflakeMap
	users                *snowflakeMap
	joinedChannels       map[string]bool // TODO: make this a list of IDs, not channels
	token                string
	guildID              string
	loggedin             bool
	clientPrefix         *irc.Prefix
	serverPrefix         *irc.Prefix
	session              *discordgo.Session
	conn                 *irc.Conn
	netConn              net.Conn
	hasRunNick           bool
	recentlySentMessages map[string][]string
}

func (user *ircUser) Close() (err error) {
	if user.session != nil {
		err = user.session.Close()
	}
	if user.netConn != nil {
		err = user.netConn.Close()
	}
	return
}

func (user *ircUser) Decode() (message *irc.Message, err error) {
	netData, err := bufio.NewReader(user.netConn).ReadString('\n')
	message = irc.ParseMessage(netData)
	if message != nil {
		fmt.Println(message)
	}
	return
}

func (user *ircUser) Encode(message *irc.Message) (err error) {
	fmt.Println(message.String())
	err = user.conn.Encode(message)
	return
}

func truncate(str string, chars int) string {
	if len(str) >= chars {
		return str[0:chars]
	}
	return str
}

func convertDiscordChannelNameToIRC(discordName string) (IRCName string) {
	re := regexp.MustCompile(`[\07\n#,]+`)
	cleaned := re.ReplaceAllString(discordName, "")
	return truncate("#"+cleaned, 50)
}

func convertDiscordUsernameToIRCUser(discordName string) (IRCUser string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\]+")
	cleaned := re.ReplaceAllString(discordName, "")

	IRCUser = truncate(cleaned, 20) // arbitrary limit, i couldn't find a real one

	if IRCUser == "" {
		IRCUser = "_"
	}

	return
}

func convertDiscordUsernameToIRCRealname(discordName string) (IRCName string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\ ]+")
	cleaned := re.ReplaceAllString(discordName, "")

	IRCName = truncate(cleaned, 20) // arbitrary limit, i couldn't find a real one

	if IRCName == "" {
		IRCName = "_"
	}

	return
}

func convertDiscordUsernameToIRCNick(discordName string) (IRCNick string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\]+")
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
