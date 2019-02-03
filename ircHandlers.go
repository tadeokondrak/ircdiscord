package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func ircWHOIS(message *irc.Message, user *ircUser) {

}

func ircLIST(message *irc.Message, user *ircUser) {
	if len(message.Params) > 0 {
		// TODO
		return
	}
	user.Encode(&irc.Message{
		Prefix:  user.serverPrefix,
		Command: irc.RPL_LISTSTART,
		Params:  []string{user.nick, "Channel", "Users  Name"},
	})
	for ircChannel, discordChannel := range user.channels {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.RPL_LIST,
			Params:  []string{user.nick, ircChannel, discordChannel.ID, convertDiscordTopicToIRC(discordChannel.Topic, user.session)},
		})
	}
	user.Encode(&irc.Message{
		Prefix:  user.serverPrefix,
		Command: irc.RPL_LISTEND,
		Params:  []string{user.nick, "End of /LIST"},
	})
}

func ircPRIVMSG(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NORECIPIENT,
			Params:  []string{"No recipient given (PRIVMSG)"},
		})
		return
	}
	if len(message.Params) < 2 || message.Params[1] == "" {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NOTEXTTOSEND,
			Params:  []string{"No text to send"},
		})
		return
	}

	channel := user.channels[message.Params[0]]
	if channel == nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NOSUCHCHANNEL,
			Params:  []string{message.Params[0], "No such channel"},
		})
		return
	}

	content := convertIRCMessageToDiscord(message.Params[1])

	addRecentlySentMessage(user, channel.ID, content)

	_, err := user.session.ChannelMessageSend(channel.ID, content)
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
		user.session.AddHandler(messageCreate)
		user.session.AddHandler(channelUpdate)
		user.session.AddHandler(channelDelete)
		user.session.AddHandler(channelCreate)
	}
	user.session = discordSessions[user.token]
	if ircSessions[user.session.Token] == nil {
		ircSessions[user.session.Token] = map[string][]*ircUser{}
	}
	ircSessions[user.session.Token][user.guildID] = append(ircSessions[user.session.Token][user.guildID], user)

	var err error
	user.discordUser, err = user.session.User("@me")
	if err != nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error getting user data from Discord."},
		})
		user.Close()
		return
	}

	user.nick = convertDiscordUsernameToIRC(getDiscordNick(user, user.discordUser))
	user.realName = convertDiscordUsernameToIRC(user.discordUser.Username)
	user.clientPrefix.Name = user.nick
	user.clientPrefix.User = user.realName
	user.clientPrefix.Host = user.discordUser.ID
	user.loggedin = true

	loadChannels(user.session, user.guildID)
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
			Params:  []string{user.nick, irc.JOIN, "Not enough parameters"},
		})
		return
	}

	for _, channelName := range strings.Split(message.Params[0], ",") {
		discordChannel, ok := user.channels[channelName]
		if !ok {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.ERR_NOSUCHCHANNEL,
				Params:  []string{channelName, "No such channel"},
			})
			continue
		}
		user.joinedChannels[channelName] = true
		user.Encode(&irc.Message{
			Prefix:  user.clientPrefix,
			Command: irc.JOIN,
			Params:  []string{channelName},
		})
		topic := convertDiscordTopicToIRC(discordChannel.Topic, user.session)
		if topic != "" {
			user.Encode(&irc.Message{
				Prefix:  user.clientPrefix,
				Command: irc.RPL_TOPIC,
				Params:  []string{user.nick, channelName, topic},
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
