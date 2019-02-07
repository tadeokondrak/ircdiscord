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
	passwordEntered      bool
	loggedin             bool
	clientPrefix         irc.Prefix
	serverPrefix         irc.Prefix
	latestPONG           string
	recentlySentMessages map[string][]string
	conn                 net.Conn
	user                 ircUser
	reader               *bufio.Reader
	sync.Mutex
}

func (c *ircConn) connect() (err error) {
	args := strings.Split(c.user.password, ":")
	if len(args) < 2 || (*serverPass != "" && len(args) < 3) { // TODO: change this when we add DM support
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

	if _, exists := guildSessions[token]; !exists {
		guildSessions[token] = make(map[string]*guildSession)
	}

	guildSession, exists := guildSessions[token][guildID]
	if !exists { // we should lock guildSessions somehow while doing this
		var err error
		guildSession, err = newGuildSession(token, guildID)
		if err != nil {
			if guildSession != nil && guildSession.session != nil {
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
		Name: c.getNick(c.self.User),
		User: convertDiscordUsernameToIRCRealname(c.self.User.Username),
		Host: c.self.User.ID,
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

	nick := c.getNick(c.self.User)

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
	if c.user.nick != "" && c.user.username != "" && c.user.realname != "" && c.user.password != "" && !c.user.capBlocked {
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
