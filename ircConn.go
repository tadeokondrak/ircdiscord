package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/tadeokondrak/irc"
)

type ircUser struct {
	nick                  string
	username              string
	realname              string
	password              string
	capBlocked            bool
	supportsCap302        bool
	supportedCapabilities map[string]bool
}

type ircConn struct {
	*guildSession
	channels             map[string]bool // map[channnelid]bool
	channelsMutex        sync.RWMutex
	passwordEntered      bool
	loggedin             bool
	clientPrefix         irc.Prefix
	serverPrefix         irc.Prefix
	latestPONG           string
	recentlySentMessages map[string][]string
	conn                 net.Conn
	user                 ircUser
	reader               *bufio.Reader
	lastPING             string
	lastPONG             string
	sync.Mutex
}

func (c *ircConn) connect() (err error) {
	args := strings.Split(c.user.password, ":")
	if len(args) < 1 || (*serverPass != "" && len(args) < 2) { // TODO: change this when we add DM support
		return errors.New("Invalid password (not enough arguments)")
	}

	if *serverPass != "" {
		if args[0] != *serverPass {
			return errors.New("Invalid password (incorrect server password)")
		}
		args = args[1:]
	}

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

	guildSession, err := getGuildSession(token, guildID)
	if err != nil {
		guildSession, err = newGuildSession(token, guildID)
		if err != nil {
			c.sendNOTICE("Failed to connect to Discord. Check if your token is correct")
			c.close()
			return err
		}
	}

	if guildSession == nil {
		return errors.New("couldn't find or create a guild session")
	}

	c.guildSession = guildSession
	c.guildSession.addConn(c)
	c.loggedin = true

	c.clientPrefix = irc.Prefix{
		Name: c.getNick(c.selfUser),
		User: convertDiscordUsernameToIRCRealname(c.selfUser.Username),
		Host: c.selfUser.ID,
	}

	return
}

func (c *ircConn) register() (err error) {
	err = c.connect()
	if err != nil {
		c.sendNOTICE(fmt.Sprint(err))
		c.close()
		return
	}

	nick := c.getNick(c.selfUser)

	if nick == "" {
		c.sendNOTICE("something with discord failed, we couldn't get a nick")
		c.close()
		return
	}
	
	guildName := strings.Replace(c.guildSession.guild.Name, " ", "-", -1)

	c.sendNICK("", "", "", nick)

	c.sendRPL(irc.RPL_WELCOME, fmt.Sprintf("Welcome to the Discord Internet Relay Chat Network %s", nick))
	c.sendRPL(irc.RPL_YOURHOST, fmt.Sprintf("Your host is %[1]s, running version IRCdiscord-%[2]s", "serverhostname", version))
	c.sendRPL(irc.RPL_CREATED, fmt.Sprintf("This server was created %s", humanize.Time(startTime)))
	c.sendRPL(irc.RPL_MYINFO, c.serverPrefix.Host, "IRCdiscord-"+version, "", "", "b")
	c.sendRPL(irc.RPL_ISUPPORT, "NICKLEN=32 MAXNICKLEN=36 AWAYLEN=0 KICKLEN=0 CHANTYPES=# NETWORK="+guildName, "are supported by this server") // TODO: change nicklen to be more accurate
	// TODO: KICKLEN is the max ban reason in discord
	// CHANNELLEN is the max channel name length
	//
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
	if c.user.nick != "" && c.user.username != "" && c.user.realname != "" && c.user.password != "" && !c.user.capBlocked {
		return true
	}
	return false
}

func (c *ircConn) close() (err error) {
	if c.guildSession != nil {
		c.guildSession.removeConn(c)
	}
	if c.conn != nil {
		err = c.conn.Close()
	}
	return
}

func (c *ircConn) decode() (message *irc.Message, err error) {
	netData, err := c.reader.ReadString('\n')
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
