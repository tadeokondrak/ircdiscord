package main

import (
	"bufio"
	"fmt"
	"net"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

const (
	version = "0.0.0-20190205-3" // TODO: update
)

var (
	startTime             = time.Now()
	supportedCapabilities = []string{}
	discordSessions       = map[string]*discordgo.Session{}
	guildSessions         = map[string]map[string]*guildSession{}
	ircSessions           = map[string]map[string][]*ircConn{}
)

func handleConnection(conn net.Conn) {
	serverHostname := conn.LocalAddr().(*net.TCPAddr).IP.String()
	clientHostname := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	c := &ircConn{
		serverPrefix: irc.Prefix{
			Name: serverHostname,
		},
		clientPrefix: irc.Prefix{
			Name: "*",
			User: "*",
			Host: clientHostname,
		},
		recentlySentMessages: make(map[string][]string),
		conn:                 conn,
		channels:             make(map[string]bool),
		user: ircUser{
			nick:                  "*",
			username:              "*",
			supportedCapabilities: make(map[string]bool),
		},
		reader: bufio.NewReader(conn),
	}

	fmt.Printf("%s connected\n", clientHostname)
	defer fmt.Printf("%s disconnected\n", clientHostname)
	defer c.close()
	for {
		message, err := c.decode()
		if err != nil { // if connection read failed
			fmt.Println(err)
			return
		}

		if message == nil { // if message parse failed
			continue
		}

		switch message.Command {
		case irc.PASS:
			c.handlePASS(message)
			continue
		case irc.CAP:
			c.handleCAP(message)
			continue
		case irc.USER:
			c.handleUSER(message)
			continue
		case irc.NICK:
			c.handleNICK(message)
			continue
		}

		if c.loggedin {
			switch message.Command {
			case irc.NICK:
				go c.handleNICK(message)
				continue
			case irc.USER:
				go c.handleUSER(message)
				continue
			case irc.PING:
				go c.handlePING(message)
				continue
			case irc.JOIN:
				go c.handleJOIN(message)
				continue
			case irc.PRIVMSG:
				go c.handlePRIVMSG(message)
				continue
			case irc.LIST:
				go c.handleLIST(message)
				continue
			case irc.PART:
				go c.handlePART(message)
				continue
			case irc.NAMES:
				go c.handleNAMES(message)
				continue
			}
		}
	}
}

func main() {
	server, err := net.Listen("tcp4", ":6667")
	if err != nil {
		fmt.Println(err)
		return
	}

	defer server.Close()
	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}
