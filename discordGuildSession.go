package main

import (
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/google/uuid"
	"github.com/tadeokondrak/IRCdiscord/snowflakemap"
)

type guildSessionType int

const (
	guildSessionDM guildSessionType = iota
	guildSessionGuild
)

type guildSession struct {
	guildSessionType
	guild         *discordgo.Guild
	session       *discordgo.Session
	selfMember    *discordgo.Member
	selfUser      *discordgo.User
	userMap       *snowflakemap.SnowflakeMap
	channelMap    *snowflakemap.SnowflakeMap
	roleMap       *snowflakemap.SnowflakeMap
	channels      map[string]*discordgo.Channel // map[channelid]Channel
	channelsMutex sync.RWMutex
	members       map[string]*discordgo.Member
	membersMutex  sync.RWMutex
	roles         map[string]*discordgo.Role
	rolesMutex    sync.RWMutex
	messages      map[string]*discordgo.Message
	messagesMutex sync.RWMutex
	users         map[string]*discordgo.User
	usersMutex    sync.RWMutex
	conns         []*ircConn
	connsMutex    sync.RWMutex
}

func newDiscordSession(token string) (session *discordgo.Session, err error) {
	session, err = discordgo.New(token)
	if err != nil {
		return nil, err
	}

	addHandlers(session)

	err = session.Open()
	if err != nil {
		return nil, err
	}

	return session, nil
}

var pingTicker = time.NewTicker(60 * time.Second)

func pingPongLoop() {
	for range pingTicker.C {
		conns := []*ircConn{}
		sessions := []*guildSession{}
		guildsessionsMutex.Lock()
		for _, sessionMap := range guildSessions {
			for _, session := range sessionMap {
				sessions = append(sessions, session)
				for _, conn := range session.conns {
					conns = append(conns, conn)
				}
			}
		}
		guildsessionsMutex.Unlock()

		// remove all conns that aren't connected
		var wg sync.WaitGroup
		wg.Add(len(conns))
		for _, c := range conns {
			go func(c *ircConn) {
				defer wg.Done()
				c.sendPING(uuid.New().String())
				time.Sleep(30 * time.Second)
				if c.lastPING != c.lastPONG {
					c.close()
				}
			}(c)
		}
		wg.Wait()

		// remove all guildSessions without a conn
		wg = sync.WaitGroup{}
		wg.Add(len(sessions))
		for _, s := range sessions {
			go func(s *guildSession) {
				defer wg.Done()
				if len(s.conns) < 1 {
					guildsessionsMutex.Lock()
					if guildSessions[s.session.Token] != nil && s.guild != nil {
						delete(guildSessions[s.session.Token], s.guild.ID)
					}
					guildsessionsMutex.Unlock()
				}
			}(s)
		}
		wg.Wait()

		// remove all discordSessions without a guildSession
		wg = sync.WaitGroup{}
		wg.Add(len(guildSessions))
		guildsessionsMutex.Lock()
		for token, sessionMap := range guildSessions {
			go func(token string, sessionMap map[string]*guildSession) {
				defer wg.Done()
				if len(sessionMap) < 1 {
					if discordSessions[token] != nil {
						discordSessions[token].Close()
					}
					delete(discordSessions, token)
					delete(sessionMap, token)
				}
			}(token, sessionMap)
		}
		guildsessionsMutex.Unlock()
		wg.Wait()
	}
}

func getGuildSession(token string, guildID string) (session *guildSession, err error) {
	guildsessionsMutex.Lock()
	if _, exists := guildSessions[token]; !exists {
		guildSessions[token] = make(map[string]*guildSession)
	}
	session, exists := guildSessions[token][guildID]
	guildsessionsMutex.Unlock()
	if !exists {
		return nil, errors.New("Couldn't find guild session")
	}
	return
}

// if guildID is empty, will return guildSession for DM server
func newGuildSession(token string, guildID string) (session *guildSession, err error) {
	discordSessionsMutex.Lock()
	discordSession, exists := discordSessions[token]
	if !exists {
		discordSession, err = newDiscordSession(token)
		if err != nil {
			return nil, err
		}
	}
	discordSessionsMutex.Unlock()

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
		userMap:          snowflakemap.NewSnowflakeMap("|"),
		roleMap:          snowflakemap.NewSnowflakeMap("@"),
		channels:         make(map[string]*discordgo.Channel),
		channelsMutex:    sync.RWMutex{},
		members:          make(map[string]*discordgo.Member),
		membersMutex:     sync.RWMutex{},
		roles:            make(map[string]*discordgo.Role),
		rolesMutex:       sync.RWMutex{},
		messages:         make(map[string]*discordgo.Message),
		messagesMutex:    sync.RWMutex{},
		users:            make(map[string]*discordgo.User),
		usersMutex:       sync.RWMutex{},
		conns:            []*ircConn{},
		connsMutex:       sync.RWMutex{},
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

	guildsessionsMutex.Lock()
	guildSessions[token][guildID] = session
	guildsessionsMutex.Unlock()
	return session, nil
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
	roles := g.guild.Roles

	for _, role := range roles {
		g.addRole(role)
	}

	return
}

func (g *guildSession) getChannel(channelID string) (channel *discordgo.Channel, err error) {
	g.channelsMutex.Lock()
	defer g.channelsMutex.Unlock()
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
	g.channelsMutex.Lock()
	g.channels[channel.ID] = channel
	g.channelsMutex.Unlock()
	if channel.Type != discordgo.ChannelTypeGuildText && channel.Type != discordgo.ChannelTypeDM && channel.Type != discordgo.ChannelTypeGroupDM {
		return ""
	}

	if channel.Name == "" && channel.Recipients != nil && len(channel.Recipients) > 0 { // DM channel
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
	g.channelsMutex.Lock()
	g.channels[channel.ID] = channel
	g.channelsMutex.Unlock()
}

func (g *guildSession) removeChannel(channel *discordgo.Channel) {
	g.channelMap.RemoveSnowflake(channel.ID)
}

func (g *guildSession) addRole(role *discordgo.Role) (name string) {
	g.rolesMutex.Lock()
	g.roles[role.ID] = role
	g.rolesMutex.Unlock()
	return g.roleMap.Add(role.Name, role.ID)
}

func (g *guildSession) updateRole(role *discordgo.Role) {
	g.rolesMutex.Lock()
	g.roles[role.ID] = role
	defer g.rolesMutex.Unlock()
}

func (g *guildSession) removeRole(roleID string) {
	g.roleMap.RemoveSnowflake(roleID)
}

func (g *guildSession) addMember(member *discordgo.Member) (name string) {
	g.membersMutex.Lock()
	g.members[member.User.ID] = member
	g.membersMutex.Unlock()
	return g.addUser(member.User)
}

func (g *guildSession) updateMember(member *discordgo.Member) {
	g.membersMutex.Lock()
	g.members[member.User.ID] = member
	g.membersMutex.Unlock()
	g.updateUser(member.User)
}

func (g *guildSession) removeMember(member *discordgo.Member) {
	g.removeUser(member.User)
}

func (g *guildSession) addUser(user *discordgo.User) (name string) {
	g.usersMutex.Lock()
	g.users[user.ID] = user
	g.usersMutex.Unlock()
	member, err := g.getMember(user.ID)
	if member != nil && err == nil && member.Nick != "" {
		return g.userMap.Add(getIRCNick(member.Nick), user.ID)
	}
	return g.userMap.Add(getIRCNick(user.Username), user.ID)
}

func (g *guildSession) updateUser(user *discordgo.User) {
	g.usersMutex.Lock()
	g.users[user.ID] = user
	g.usersMutex.Unlock()
}

func (g *guildSession) removeUser(user *discordgo.User) {
	g.userMap.RemoveSnowflake(user.ID)
}

func (g *guildSession) addMessage(message *discordgo.Message) {
	g.messagesMutex.Lock()
	g.messages[message.ID] = message
	g.messagesMutex.Unlock()
}

func (g *guildSession) updateMessage(message *discordgo.Message) {
	g.addMessage(message)
}

func (g *guildSession) getMessage(channelID string, messageID string) (message *discordgo.Message, err error) {
	g.messagesMutex.Lock()
	defer g.messagesMutex.Unlock()
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

func (g *guildSession) getRole(roleID string) (role *discordgo.Role, err error) {
	g.rolesMutex.Lock()
	defer g.rolesMutex.Unlock()
	role, exists := g.roles[roleID]
	if exists {
		return
	}

	if g.session == nil || g.guild == nil {
		return nil, err
	}

	roles, err := g.session.GuildRoles(g.guild.ID)
	if err != nil {
		return nil, err
	}
	for _, _role := range roles {
		g.addRole(_role)
	}

	role, exists = g.roles[roleID]
	if exists {
		return
	}

	return nil, err
}

func (g *guildSession) getMember(userID string) (member *discordgo.Member, err error) {
	g.membersMutex.Lock()
	defer g.membersMutex.Unlock()
	member, exists := g.members[userID]
	if exists {
		return
	}

	if g.session == nil || g.guild == nil {
		return nil, err
	}

	member, err = g.session.GuildMember(g.guild.ID, userID)
	if err != nil {
		return nil, err
	}

	g.members[userID] = member
	return
}

func (g *guildSession) getNick(user *discordgo.User) (nick string) {
	if user == nil {
		return ""
	}

	if user.Discriminator == "0000" { // webhooks don't have nicknames
		return getIRCNick(user.Username) + "|w"
	}

	nick = g.userMap.GetName(user.ID)
	if nick != "" {
		return
	}

	g.addUser(user)
	return g.userMap.GetName(user.ID)
}

func (g *guildSession) getRealname(user *discordgo.User) (realname string) {
	return g.getNick(user)
}

func (g *guildSession) getChannelName(channel *discordgo.Channel) (channelname string) {
	return g.channelMap.GetName(channel.ID)
}

func (g *guildSession) getRoleName(role *discordgo.Role) (rolename string) {
	return g.roleMap.GetName(role.ID)
}

func (g *guildSession) addConn(conn *ircConn) {
	g.connsMutex.Lock()
	g.conns = append(g.conns, conn)
	g.connsMutex.Unlock()
}

func (g *guildSession) removeConn(conn *ircConn) {
	g.connsMutex.Lock()
	for i, _conn := range g.conns {
		if conn == _conn {
			g.conns = append(g.conns[:i], g.conns[i+1:]...)
			break
		}
	}
	g.connsMutex.Unlock()
}
