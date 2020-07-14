package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/tadeokondrak/ircdiscord/internal/client"
)

var (
	ircDebug     bool
	discordDebug bool
	tlsEnabled   bool
	port         int
	certfile     string
	keyfile      string
)

func runClient(conn net.Conn) {
	c := client.New(conn)
	c.IRCDebug = ircDebug
	c.DiscordDebug = discordDebug
	if err := c.Run(); err != nil {
		log.Println(err)
	}
}

func main() {
	flag.BoolVar(&ircDebug, "ircdebug", false,
		"enable logging of all irc communication")
	flag.BoolVar(&discordDebug, "discorddebug", false,
		"enable logging of some discord communication")
	flag.BoolVar(&tlsEnabled, "tls", false, "enable tls encryption")
	flag.IntVar(&port, "port", 0,
		"port to run on, defaults to 6667/6697 depending on tls")
	flag.StringVar(&certfile, "cert", "", "tls certificate file")
	flag.StringVar(&keyfile, "key", "", "tls key file")
	flag.Parse()

	log.SetFlags(0)

	var listener net.Listener
	if tlsEnabled {
		if certfile == "" || keyfile == "" {
			log.Fatalf("certfile/keyfile required for tls")
		}

		var cert tls.Certificate
		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			log.Fatalf("failed to load keypair: %v", err)
		}

		config := tls.Config{
			Certificates: []tls.Certificate{cert},
		}

		if port == 0 {
			port = 6697
		}

		addr := fmt.Sprintf(":%d", port)

		listener, err = tls.Listen("tcp", addr, &config)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
	} else {
		if port == 0 {
			port = 6667
		}

		var err error
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Fatalf("failed to create listener: %v", err)
		}
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}
		go runClient(conn)
	}
}
