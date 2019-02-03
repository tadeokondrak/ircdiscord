package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func ircWHOIS(message *irc.Message, user *ircUser) {

}

func ircPRIVMSG(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NORECIPIENT,
			Params:  []string{"No recipient given (PRIVMSG)"},
		})
	}
	if len(message.Params) < 2 || message.Params[1] == "" {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NOTEXTTOSEND,
			Params:  []string{"No text to send"},
		})
	}
	var channelID string
	for id, name := range user.channels {
		if name == message.Params[0] {
			channelID = id
			break
		}
	}
	if channelID == "" {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NOSUCHCHANNEL,
			Params:  []string{message.Params[0], "No such channel"},
		})
	}

	content := convertIRCMessageToDiscord(message.Params[1])

	addRecentlySentMessage(user, channelID, content)

	_, err := user.session.ChannelMessageSend(channelID, content)
	if err != nil {
		// TODO: map common discord errors to irc errors
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error sending your message."},
		})
		fmt.Println(err)
		return
	}
}

func ircUSER(message *irc.Message, user *ircUser) {

}

func ircPASS(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NEEDMOREPARAMS,
			Params:  []string{user.nick, irc.PASS, "Not enough parameters"},
		})
		return
	}
	args := strings.Split(message.Params[0], ":")
	user.token = args[0]
	if len(args) < 2 {
		user.guildID = "DM"
	} else {
		user.guildID = args[1]
	}

	if _, ok := discordSessions[user.token]; !ok { // if token does not exist in discordSessions
		session, err := discordgo.New(user.token)
		if err != nil {
			delete(discordSessions, user.token)
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.NOTICE,
				Params:  []string{user.nick, "Failed to create Discord session. Check if your token is correct"},
			})
			user.Close()
			return
		}
		discordSessions[user.token] = session
		user.session = session
		user.session.StateEnabled = true
		err = user.session.Open()
		if err != nil {
			delete(discordSessions, user.token)
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.NOTICE,
				Params:  []string{user.nick, "Failed to connect to Discord. Check if your token is correct"},
			})
			user.Close()
			return
		}
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "Successfully connected to Discord!"},
		})
		user.session.AddHandler(messageCreate)
	}
	user.session = discordSessions[user.token]
	if ircSessions[user.session.Token] == nil {
		ircSessions[user.session.Token] = map[string][]*ircUser{}
	}
	ircSessions[user.session.Token][user.guildID] = append(ircSessions[user.session.Token][user.guildID], user)
	discordUser, err := user.session.User("@me")
	if err != nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error getting user data from Discord."},
		})
		return
	}
	user.nick = convertDiscordUsernameToIRC(getDiscordNick(user, discordUser))
	user.realName = convertDiscordUsernameToIRC(discordUser.Username)
	user.clientPrefix.Name = user.nick
	user.clientPrefix.User = user.realName
	user.clientPrefix.Host = discordUser.ID
	user.loggedin = true
	user.Encode(&irc.Message{
		Prefix:  user.serverPrefix,
		Command: irc.RPL_WELCOME,
		Params: []string{
			user.nick,
			fmt.Sprintf("Welcome to the Discord Internet Relay Chat Network %s", user.nick),
		},
	})
}

func ircJOIN(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		// ERR_NEEDMOREPARAMS              ERR_BANNEDFROMCHAN
		// ERR_INVITEONLYCHAN              ERR_BADCHANNELKEY
		// ERR_CHANNELISFULL               ERR_BADCHANMASK
		// ERR_NOSUCHCHANNEL               ERR_TOOMANYCHANNELS
		// RPL_TOPIC
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NEEDMOREPARAMS,
			Params: []string{
				user.nick,
				irc.JOIN,
				"Not enough parameters"},
		})
		return
	}
	channelsToJoin := strings.Split(message.Params[0], ",")

	guildChannels, err := user.session.GuildChannels(user.guildID)
	if err != nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error getting channels from Discord."},
		})
		return
	}

	guildChannelMap := make(map[string]*discordgo.Channel)
	for _, channel := range guildChannels {
		guildChannelMap["#"+channel.Name] = channel
	}

	for _, channelName := range channelsToJoin {
		if channel, ok := guildChannelMap[channelName]; ok {
			user.channels[channel.ID] = channelName
			user.Encode(&irc.Message{
				Prefix:  user.clientPrefix,
				Command: irc.JOIN,
				Params:  []string{"#" + channel.Name},
			})
		} else {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.ERR_NOSUCHCHANNEL,
				Params:  []string{channelName, "No such channel"},
			})
		}
	}
}

func ircPING(message *irc.Message, user *ircUser) {
	user.Encode(&irc.Message{
		Prefix:  user.serverPrefix,
		Command: irc.PONG,
		Params:  message.Params,
	})
}

func ircNICK(message *irc.Message, user *ircUser) {
	if !user.hasRunNick {
		user.hasRunNick = true
		return
	}

	if len(message.Params) < 1 {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NONICKNAMEGIVEN,
			Params:  []string{user.nick, "No nickname given"},
		})
		return
	}
	if !isValidDiscordNick(message.Params[0]) {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_ERRONEUSNICKNAME,
			Params:  []string{user.nick, "Erroneus nickname"},
		})
		return
	}
	// user.nick = message.Params[0]
}
