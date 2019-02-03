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
	channels             map[string]string
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
	err = user.netConn.Close()
	return
}

func (user *ircUser) Decode() (message *irc.Message, err error) {
	netData, err := bufio.NewReader(user.netConn).ReadString('\n')
	message = irc.ParseMessage(netData)
	return
}

func (user *ircUser) Encode(message *irc.Message) (err error) {
	fmt.Println(message.String())
	err = user.conn.Encode(message)
	return
}

func convertDiscordUsernameToIRC(discordName string) (IRCNick string) {
	re := regexp.MustCompile("[^a-zA-Z0-9\\[\\]\\{\\}\\^_\\-|`\\\\]+")
	cleaned := re.ReplaceAllString(discordName, "")
	if len(cleaned) >= 9 {
		IRCNick = cleaned[0:9]
	} else {
		IRCNick = cleaned
	}
	return
}

func convertDiscordContentToIRC(discordContent string, session *discordgo.Session) (ircContent string) {
	return discordContent
}
