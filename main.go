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
	cl := client.New(conn, ircDebug, discordDebug)
	defer cl.Close()

	if err := cl.Run(); err != nil {
		log.Println(err)
	}
}

func runServer(ln net.Listener) {
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}

		go runClient(conn)
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

	if tlsEnabled {
		if certfile == "" || keyfile == "" {
			log.Fatalf("certfile and keyfile are required for TLS")
		}

		cert, err := tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			log.Fatalf("failed to load keypair: %v", err)
		}

		config := &tls.Config{Certificates: []tls.Certificate{cert}}

		if port == 0 {
			port = 6697
		}
		addr := fmt.Sprintf(":%d", port)

		listener, err := tls.Listen("tcp", addr, config)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		runServer(listener)
	} else {
		if port == 0 {
			port = 6667
		}
		addr := fmt.Sprintf(":%d", port)

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("failed to listen: %v", err)
		}

		runServer(listener)
	}
}
