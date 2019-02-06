package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"gopkg.in/sorcix/irc.v2"
)

const (
	version = "0.0.0-20190205-7" // TODO: update
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

var (
	serverPass = flag.String("serverpassword", "", "Server password that must also be specified when logging in.")
)

func main() {
	tlsEnabled := flag.Bool("tls", false, "Enable TLS encrypted connections.")
	portFlag := flag.Int("port", 0, "Port to listen on. If unspecified, will listen on 6667 if TLS is disabled or 6697 if enabled.")
	address := flag.String("address", "127.0.0.1", "Address to listen on. Set to \"0.0.0.0\" to listen on all interfaces, leave default if you're connecting from the same computer as the server.")
	certfile := flag.String("certfile", "", "For TLS: certificate file.")
	keyfile := flag.String("keyfile", "", "For TLS: key file.")
	flag.Parse()

	if *tlsEnabled && (*certfile == "" || *keyfile == "") {
		log.Fatalln("certfile and keyfile must be specified if tls is enabled")
	}

	port := strconv.Itoa(*portFlag)

	var err error
	var server net.Listener
	if *tlsEnabled {
		if port == "0" {
			port = "6697"
		}
		cert, err := tls.LoadX509KeyPair(*certfile, *keyfile)
		if err != nil {
			log.Fatalln(err)
		}
		server, err = tls.Listen("tcp", *address+":"+port, &tls.Config{Certificates: []tls.Certificate{cert}})
	} else {
		if port == "0" {
			port = "6667"
		}
		server, err = net.Listen("tcp", *address+":"+port)
	}
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
