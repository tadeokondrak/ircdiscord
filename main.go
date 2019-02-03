package main

import (
	"fmt"
	"net"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

var (
	discordSessions = map[string]*discordgo.Session{}
	ircSessions     = map[string]map[string][]*ircUser{} // ircSessions[token][guildid] is a slice of ircUsers
)

func handleConnection(conn net.Conn) {
	serverHostname := conn.LocalAddr().(*net.TCPAddr).IP.String()
	clientHostname := conn.RemoteAddr().(*net.TCPAddr).IP.String()
	user := &ircUser{
		nick:     "*",
		hostname: clientHostname,
		serverPrefix: &irc.Prefix{
			Name: serverHostname,
		},
		clientPrefix: &irc.Prefix{
			Name: "*",
			User: "*",
			Host: clientHostname,
		},
		conn:     irc.NewConn(conn),
		netConn:  conn,
		channels: make(map[string]string),
	}

	fmt.Printf("%s connected\n", clientHostname)
	defer fmt.Printf("%s disconnected\n", clientHostname)
	defer user.Close()
	for {
		message, err := user.Decode()
		if err != nil { // if connection read failed
			fmt.Println(err)
			return
		}

		if message == nil { // if message parse failed
			continue
		}

		if user.loggedin {
			switch message.Command {
			case irc.NICK:
				go ircNICK(message, user)
			case irc.USER:
				go ircUSER(message, user)
			case irc.PING:
				go ircPING(message, user)
			case irc.JOIN:
				go ircJOIN(message, user)
			case irc.PRIVMSG:
				go ircPRIVMSG(message, user)
			case irc.WHOIS:
				go ircWHOIS(message, user)
			}
		} else {
			if message.Command == irc.PASS {
				ircPASS(message, user)
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
