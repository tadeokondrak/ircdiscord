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
	selfMember *discordgo.Member
	selfUser   *discordgo.User
	userMap    *snowflakemap.SnowflakeMap
	channelMap *snowflakemap.SnowflakeMap
	roleMap    *snowflakemap.SnowflakeMap
	channels   map[string]*discordgo.Channel // map[channelid]Channel
	members    map[string]*discordgo.Member
	roles      map[string]*discordgo.Role
	messages   map[string]*discordgo.Message
	users      map[string]*discordgo.User
	lastAck    *discordgo.Ack
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

	selfUser, err := discordSession.User("@me")
	if err != nil {
		return
	}

	var selfMember *discordgo.Member
	if sessionType == guildSessionGuild {
		selfMember, err = discordSession.GuildMember(guild.ID, selfUser.ID)
		if err != nil {
			return
		}
	}

	session = &guildSession{
		guildSessionType: sessionType,
		guild:            guild,
		session:          discordSession,
		selfMember:       selfMember,
		selfUser:         selfUser,
		channelMap:       snowflakemap.NewSnowflakeMap("#"),
		userMap:          snowflakemap.NewSnowflakeMap("`"),
		roleMap:          snowflakemap.NewSnowflakeMap("@"),
		channels:         make(map[string]*discordgo.Channel),
		members:          make(map[string]*discordgo.Member),
		roles:            make(map[string]*discordgo.Role),
		messages:         make(map[string]*discordgo.Message),
		users:            make(map[string]*discordgo.User),
		lastAck:          &discordgo.Ack{},
		conns:            []*ircConn{},
	}

	err = session.populateChannelMap()
	if err != nil {
		return nil, err
	}

	if sessionType == guildSessionGuild {
		err = session.populateUserMap("")
		if err != nil {
			return nil, err
		}

		err = session.populateRoleMap()
		if err != nil {
			return nil, err
		}
	}

	return
}

func (g *guildSession) populateChannelMap() (err error) {
	var channels []*discordgo.Channel
	if g.guildSessionType == guildSessionGuild {
		channels, err = g.session.GuildChannels(g.guild.ID)
		if err != nil {
			return err
		}
	} else if g.guildSessionType == guildSessionDM {
		channels, err = g.session.UserChannels()
		if err != nil {
			return err
		}
	}

	for _, channel := range channels {
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
		g.addMember(member)
	}

	return
}

func (g *guildSession) populateRoleMap() (err error) {
	roles, err := g.session.GuildRoles(g.guild.ID)
	if err != nil {
		return err
	}

	for _, role := range roles {
		g.addRole(role)
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
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeDM && channel.Type != discordgo.ChannelTypeGroupDM {
		return ""
	}

	if channel.Recipients != nil && len(channel.Recipients) > 0 { // DM channel
		if len(channel.Recipients) == 1 {
			name = g.getNick(channel.Recipients[0])
		} else {
			for _, user := range channel.Recipients {
				name = name + g.getNick(user) + "&"
			}
			name = convertDiscordChannelNameToIRC(name[:len(name)-1])
		}
	} else {
		name = convertDiscordChannelNameToIRC(channel.Name)
	}

	return g.channelMap.Add(name, channel.ID)
}

func (g *guildSession) updateChannel(channel *discordgo.Channel) {
	g.channels[channel.ID] = channel
}

func (g *guildSession) removeChannel(channel *discordgo.Channel) {
	g.channelMap.RemoveSnowflake(channel.ID)
}

func (g *guildSession) addRole(role *discordgo.Role) (name string) {
	g.roles[role.ID] = role
	return g.roleMap.Add(role.Name, role.ID)
}

func (g *guildSession) updateRole(role *discordgo.Role) {
	g.roles[role.ID] = role
}

func (g *guildSession) removeRole(roleID string) {
	g.roleMap.RemoveSnowflake(roleID)
}

func (g *guildSession) addMember(member *discordgo.Member) (name string) {
	g.members[member.User.ID] = member
	return g.addUser(member.User)
}

func (g *guildSession) updateMember(member *discordgo.Member) {
	g.members[member.User.ID] = member
	g.updateUser(member.User)
}

func (g *guildSession) removeMember(member *discordgo.Member) {
	g.removeUser(member.User)
}

func (g *guildSession) addUser(user *discordgo.User) (name string) {
	g.users[user.ID] = user
	return g.userMap.Add(convertDiscordUsernameToIRCNick(user.Username), user.ID)
}

func (g *guildSession) updateUser(user *discordgo.User) {
	g.users[user.ID] = user
}

func (g *guildSession) removeUser(user *discordgo.User) {
	g.userMap.RemoveSnowflake(user.ID)
}

func (g *guildSession) addMessage(message *discordgo.Message) {
	g.messages[message.ID] = message
}

func (g *guildSession) updateMessage(message *discordgo.Message) {
	g.addMessage(message)
}

func (g *guildSession) getMessage(channelID string, messageID string) (message *discordgo.Message, err error) {
	message, exists := g.messages[messageID]
	if exists {
		return
	}

	message, err = g.session.ChannelMessage(g.guild.ID, messageID)
	if err != nil {
		return nil, err
	}

	g.messages[messageID] = message
	return
}

func (g *guildSession) getUser(userID string) (user *discordgo.User, err error) {
	member, err := g.getMember(userID)
	if err != nil {
		return
	}
	return member.User, nil
}

func (g *guildSession) getMember(userID string) (member *discordgo.Member, err error) {
	// TODO: find all functions that search g.members itself
	member, exists := g.members[userID]
	if exists {
		return
	}

	member, err = g.session.GuildMember(g.guild.ID, userID)
	if err != nil {
		return nil, err
	}

	g.members[userID] = member
	return
}

func (g *guildSession) getNick(discordUser *discordgo.User) (nick string) {
	if discordUser == nil {
		return ""
	}

	if discordUser.Discriminator == "0000" { // webhooks don't have nicknames
		return convertDiscordUsernameToIRCNick(discordUser.Username) + "`w"
	}

	nick = g.userMap.GetName(discordUser.ID)
	if nick != "" {
		return
	}

	nick = g.addUser(discordUser)
	return g.userMap.GetName(discordUser.ID)
}
