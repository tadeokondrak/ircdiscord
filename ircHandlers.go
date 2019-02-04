package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

func ircWHOIS(message *irc.Message, user *ircUser) {

}

func ircNAMES(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		// TODO: show names for every channel the user's on
		return
	}
	for ircNick := range user.users.getMap() {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.RPL_NAMREPLY,
			Params:  []string{user.nick, message.Params[0], ircNick},
		})
	}
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
	for ircChannel, discordChannelID := range user.channels.getMap() {
		discordChannel, err := user.session.State.Channel(discordChannelID)
		if err != nil {
			discordChannel, err = user.session.Channel(discordChannelID)
			if err != nil {
				fmt.Println("error fetching channel data")
				continue
			}
		}
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.RPL_LIST,
			Params:  []string{user.nick, ircChannel, discordChannelID, convertDiscordTopicToIRC(discordChannel.Topic, user.session)},
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

	channel := user.channels.get(message.Params[0])
	if channel == "" {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NOSUCHCHANNEL,
			Params:  []string{message.Params[0], "No such channel"},
		})
		return
	}

	content := convertIRCMessageToDiscord(message.Params[1])

	addRecentlySentMessage(user, channel, content)

	_, err := user.session.ChannelMessageSend(channel, content)
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
		user.session.AddHandler(messageUpdate)
		user.session.AddHandler(messageDelete)

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

	user.nick = user.users.getNick(user.discordUser)
	user.realName = convertDiscordUsernameToIRCRealname(user.discordUser.Username)
	user.clientPrefix.Name = user.nick
	user.clientPrefix.User = user.realName
	user.clientPrefix.Host = user.discordUser.ID
	user.loggedin = true

	user.channels = newSnowflakeMap()
	channels, err := user.session.GuildChannels(user.guildID)
	if err != nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error getting channels from Discord."},
		})
	}
	for _, channel := range channels {
		go user.channels.addChannel(channel)
	}

	members, err := user.session.GuildMembers(user.guildID, "", 1000)
	if err != nil {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.NOTICE,
			Params:  []string{user.nick, "There was an error getting users from Discord."},
		})
	}
	for _, member := range members {
		user.users.addUser(member.User)
	}

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
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NEEDMOREPARAMS,
			Params:  []string{user.nick, irc.JOIN, "Not enough parameters"},
		})
		return
	}

	for _, channelName := range strings.Split(message.Params[0], ",") {
		discordChannelID := user.channels.get(channelName)
		if discordChannelID == "" {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.ERR_NOSUCHCHANNEL,
				Params:  []string{channelName, "No such channel"},
			})
			continue
		}
		discordChannel, err := user.session.State.Channel(discordChannelID)
		if err != nil {
			discordChannel, err = user.session.Channel(discordChannelID)
			if err != nil {
				fmt.Println("error fetching channel data")
				continue
			}
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
		go func(user *ircUser, channel *discordgo.Channel) {
			messages, err := user.session.ChannelMessages(channel.ID, 100, "", "", "")
			if err != nil {
				user.Encode(&irc.Message{
					Prefix:  user.serverPrefix,
					Command: irc.NOTICE,
					Params:  []string{user.nick, "There was an error getting messages from Discord."},
				})
				return
			}
			for i := len(messages); i != 0; i-- { // Discord sends them in reverse order
				sendMessageFromDiscordToIRC(user, messages[i-1], "") // TODO: maybe prefix with date
			}
		}(user, discordChannel)
		go ircNAMES(&irc.Message{Command: irc.NAMES, Params: []string{channelName}}, user)
	}
}

func ircPART(message *irc.Message, user *ircUser) {
	if len(message.Params) < 1 {
		user.Encode(&irc.Message{
			Prefix:  user.serverPrefix,
			Command: irc.ERR_NEEDMOREPARAMS,
			Params:  []string{user.nick, irc.JOIN, "Not enough parameters"},
		})
		return
	}

	for _, channelName := range strings.Split(message.Params[0], ",") {
		discordChannelID := user.channels.get(channelName)
		if discordChannelID == "" {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.ERR_NOSUCHCHANNEL,
				Params:  []string{channelName, "No such channel"},
			})
			continue
		}
		if _, exists := user.joinedChannels[channelName]; !exists {
			user.Encode(&irc.Message{
				Prefix:  user.serverPrefix,
				Command: irc.ERR_NOTONCHANNEL,
				Params:  []string{channelName, "You're not on that channel"},
			})
			continue
		}
		user.joinedChannels[channelName] = false
		user.Encode(&irc.Message{
			Prefix:  user.clientPrefix,
			Command: irc.PART,
			Params:  []string{channelName},
		})
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
