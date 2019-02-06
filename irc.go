package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"

	"github.com/tadeokondrak/IRCdiscord/snowflakemap"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
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

type ircUser struct {
	nick     string
	username string
	realname string
	password string
}

type ircConn struct {
	*guildSession
	channels             map[string]bool // map[channnelid]bool
	passwordEntered      bool
	loggedin             bool
	clientPrefix         irc.Prefix
	serverPrefix         irc.Prefix
	latestPONG           string
	recentlySentMessages map[string][]string
	conn                 net.Conn
	user                 ircUser
	sync.Mutex
}

func (c *ircConn) connect() (err error) {
	args := strings.Split(c.user.password, ":")
	token := args[0]

	var guildID string
	var fields []string
	if len(args) > 1 {
		fields = append(fields, args[1])
	}
	fields = append(fields, c.user.nick, c.user.realname)
	for _, field := range fields {
		if _, err := strconv.Atoi(field); err == nil && len(field) >= 18 {
			guildID = field
			break
		}
	}

	if _, exists := guildSessions[token]; !exists {
		guildSessions[token] = make(map[string]*guildSession)
	}

	guildSession, exists := guildSessions[token][guildID]
	if !exists { // we should lock guildSessions somehow while doing this
		var err error
		guildSession, err = newGuildSession(token, guildID)
		if err != nil {
			if guildSession.session != nil {
				guildSession.session.Close()
			}
			c.sendNOTICE("Failed to connect to Discord. Check if your token is correct")
			c.close()
			return err
		}
		guildSessions[token][guildID] = guildSession
	}

	if guildSession == nil {
		return errors.New("couldn't find or create a guild session")
	}

	c.guildSession = guildSession
	guildSession.conns = append(guildSession.conns, c) // FIXME: this should be a function with a mutex
	c.loggedin = true

	c.clientPrefix = irc.Prefix{
		Name: c.guildSession.getNick(c.self),
		User: convertDiscordUsernameToIRCRealname(c.self.Username),
		Host: c.self.ID,
	}

	print("here")

	return
}

func (c *ircConn) register() (err error) {
	err = c.connect()
	if err != nil {
		c.sendNOTICE(fmt.Sprint(err))
		c.close()
		return
	}

	nick := c.guildSession.getNick(c.guildSession.self)

	if nick == "" {
		c.sendNOTICE("something with discord failed, we couldn't get a nick")
		c.close()
		return
	}
	c.sendNICK("", "", "", nick)

	c.sendRPL(irc.RPL_WELCOME, fmt.Sprintf("Welcome to the Discord Internet Relay Chat Network %s", nick))
	c.sendRPL(irc.RPL_YOURHOST, fmt.Sprintf("Your host is %[1]s, running version %[2]s", "serverhostname", "ircdiscord-version"))
	c.sendRPL(irc.RPL_CREATED, fmt.Sprintf("This server was created %s", humanize.Time(startTime)))
	c.sendRPL(irc.RPL_MYINFO, c.serverPrefix.Host, "ircdiscord-"+version, "", "", "b")
	c.sendRPL(irc.RPL_ISUPPORT, "NICKLEN=10", "are supported by this server") // TODO: change nicklen to be more accurate
	// The server SHOULD then respond as though the client sent the LUSERS command and return the appropriate numerics
	// c.handleLUSERS()
	// If the user has client modes set on them automatically upon joining the network, the server SHOULD send the client the RPL_UMODEIS (221) reply.
	//
	// The server MAY send other numerics and messages. The server MUST then respond as though the client sent it the MOTD command, i.e. it must send either the successful Message of the Day numerics or the ERR_NOMOTD numeric.
	c.handleMOTD()
	if err != nil {
		c.sendNOTICE(fmt.Sprint(err))
		c.close()
		return
	}

	return
}

func (c *ircConn) readyToRegister() bool {
	if c.user.nick != "" && c.user.username != "" && c.user.realname != "" {
		return true
	}
	return false
}

func (c *ircConn) close() (err error) {
	if c.guildSession != nil {
		// TODO: split this into its own function, with a mutex
		for index, conn := range c.guildSession.conns {
			if conn == c {
				c.guildSession.conns = append(c.guildSession.conns[index:], c.guildSession.conns[index+1:]...)
			}
		}
		if c.guildSession.session != nil && len(c.guildSession.conns) == 0 {
			err = c.guildSession.session.Close()
		}
	}
	if c.conn != nil {
		err = c.conn.Close()
	}
	return
}

func (c *ircConn) decode() (message *irc.Message, err error) {
	netData, err := bufio.NewReader(c.conn).ReadString('\n')
	message = irc.ParseMessage(netData)
	if message != nil {
		fmt.Println(message)
	}
	return
}

func (c *ircConn) encode(message *irc.Message) (err error) {
	fmt.Println(message.String())
	_, err = c.write(message.Bytes())
	return
}

func (c *ircConn) write(p []byte) (n int, err error) {
	c.Lock()
	defer c.Unlock()
	n, err = c.conn.Write(p)
	_, err = c.conn.Write([]byte("\r\n"))
	return
}

func (c *ircConn) sendNOTICE(message string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.NOTICE,
		Params:  append([]string{c.clientPrefix.Name}, message),
	})
	return
}
func (c *ircConn) sendERR(command string, params ...string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: command,
		Params:  append([]string{c.clientPrefix.Name}, params...),
	})
	return
}

func (c *ircConn) sendRPL(command string, params ...string) (err error) {
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: command,
		Params:  append([]string{c.clientPrefix.Name}, params...),
	})
	return
}

func (c *ircConn) sendJOIN(nick string, realname string, hostname string, target string, key string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.clientPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	params := []string{target}
	if key != "" {
		params = append(params, key)
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.JOIN,
		Params:  params,
	})
	return
}

func (c *ircConn) sendPART(nick string, realname string, hostname string, target string, reason string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.clientPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	params := []string{target}
	if reason != "" {
		params = append(params, reason)
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.PART,
		Params:  params,
	})
	return
}

func (c *ircConn) sendNICK(nick string, realname string, hostname string, newNick string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.serverPrefix

	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.NICK,
		Params:  []string{newNick},
	})
	return
}

func (c *ircConn) sendPONG(message string) (err error) {
	params := []string{}
	if message != "" {
		params = append(params, message)
	}
	err = c.encode(&irc.Message{
		Prefix:  &c.serverPrefix,
		Command: irc.PONG,
		Params:  params,
	})
	return
}
func (c *ircConn) sendPRIVMSG(nick string, realname string, hostname string, target string, content string) (err error) {
	var prefix *irc.Prefix
	if nick == "" || realname == "" || hostname == "" {
		prefix = &c.serverPrefix
	} else {
		prefix = &irc.Prefix{
			User: nick,
			Name: realname,
			Host: hostname,
		}
	}
	err = c.encode(&irc.Message{
		Prefix:  prefix,
		Command: irc.PRIVMSG,
		Params:  []string{target, content},
	})
	return
}

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
