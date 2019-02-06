package main

import (
	"github.com/bwmarrin/discordgo"
	"github.com/tadeokondrak/IRCdiscord/snowflakemap"
)

type guildSessionType int

const (
	guildSessionDM guildSessionType = iota
	guildSessionGuild
)

type guildSession struct {
	guildSessionType
	guild      *discordgo.Guild
	session    *discordgo.Session
	self       *discordgo.User
	userMap    *snowflakemap.SnowflakeMap
	channelMap *snowflakemap.SnowflakeMap
	channels   map[string]*discordgo.Channel // map[channelid]Channel
	users      map[string]*discordgo.User    // map[userid]User
	conns      []*ircConn
}

// if guildID is empty, will return guildSession for DM server
func newGuildSession(token string, guildID string) (session *guildSession, err error) {
	discordSession, exists := discordSessions[token]
	if !exists {
		discordSession, err = discordgo.New(token)
		if err != nil {
			return nil, err
		}

		addHandlers(discordSession)

		err = discordSession.Open()
		if err != nil {
			return nil, err
		}

		discordSessions[token] = discordSession
	}

	var guild *discordgo.Guild
	var sessionType guildSessionType
	if guildID != "" {
		sessionType = guildSessionGuild
		guild, err = discordSession.Guild(guildID)
		if err != nil {
			return
		}
	} else {
		sessionType = guildSessionDM
	}

	self, err := discordSession.User("@me")
	if err != nil {
		return
	}

	session = &guildSession{
		guildSessionType: sessionType,
		guild:            guild,
		session:          discordSession,
		self:             self,
		channelMap:       snowflakemap.NewSnowflakeMap("#"),
		userMap:          snowflakemap.NewSnowflakeMap("`"),
		channels:         make(map[string]*discordgo.Channel),
		users:            make(map[string]*discordgo.User),
		conns:            []*ircConn{},
	}

	if guild == nil {
		return nil, err
	}

	err = session.populateChannelMap()
	if err != nil {
		return nil, err
	}

	err = session.populateUserMap("")
	if err != nil {
		return nil, err
	}

	return
}

func (g *guildSession) populateChannelMap() (err error) {
	channels, err := g.session.GuildChannels(g.guild.ID)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		g.channels[channel.ID] = channel
		g.addChannel(channel)
	}

	return
}

func (g *guildSession) populateUserMap(after string) (err error) {
	members, err := g.session.GuildMembers(g.guild.ID, after, 1000)
	if err != nil {
		return err
	}

	if len(members) == 1000 {
		g.populateUserMap(members[999].User.ID)
	}

	for _, member := range members {
		g.users[member.User.ID] = member.User
		g.addUser(member.User)
	}

	return
}

func (g *guildSession) getChannel(channelID string) (channel *discordgo.Channel, err error) {
	channel, exists := g.channels[channelID]
	if exists {
		return
	}

	channel, err = g.session.Channel(channelID)
	if err != nil {
		return nil, err
	}

	g.channels[channelID] = channel
	return
}

func (g *guildSession) addChannel(channel *discordgo.Channel) (name string) {
	g.channels[channel.ID] = channel
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeDM {
		return ""
	}
	return g.channelMap.Add(convertDiscordChannelNameToIRC(channel.Name), channel.ID)
}

func (g *guildSession) updateChannel(channel *discordgo.Channel) {
	g.channels[channel.ID] = channel
}

func (g *guildSession) removeChannel(channel *discordgo.Channel) {
	g.channelMap.RemoveSnowflake(channel.ID)
}

func (g *guildSession) addUser(user *discordgo.User) (name string) {
	if user.Discriminator == "0000" {
		// We don't add users for webhooks because they're not users
		return ""
	}
	return g.userMap.Add(convertDiscordUsernameToIRCNick(user.Username), user.ID)
}

func (g *guildSession) getUser(userID string) (user *discordgo.User, err error) {
	channel, exists := g.users[userID]
	if exists {
		return
	}

	channel, err = g.session.User(userID)
	if err != nil {
		return nil, err
	}

	g.users[userID] = channel
	return
}

func (g *guildSession) getNick(discordUser *discordgo.User) (nick string) {
	if discordUser == nil {
		return ""
	}

	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return convertDiscordUsernameToIRCNick(discordUser.Username) + "`w"
	}

	return g.userMap.GetName(discordUser.ID)
}
