package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"

	ircdiscord "github.com/tadeokondrak/ircdiscord"
)

func main() {
	var (
		debug      bool
		tlsEnabled bool
		port       int
		certfile   string
		keyfile    string
	)
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.BoolVar(&tlsEnabled, "tls", false, "enable tls encryption")
	flag.IntVar(&port, "port", 0, "port to run on, defaults to 6667/6697 depending on tls")
	flag.StringVar(&certfile, "cert", "", "tls certificate file")
	flag.StringVar(&keyfile, "key", "", "tls key file")
	flag.Parse()

	log.SetFlags(0)

	var listener net.Listener
	var err error
	if tlsEnabled {
		if certfile == "" || keyfile == "" {
			log.Fatalf("certfile/keyfile required for tls")
		}
		var cert tls.Certificate
		cert, err = tls.LoadX509KeyPair(certfile, keyfile)
		if err != nil {
			log.Fatalf("failed to load keypair: %v", err)
		}
		config := tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		if port == 0 {
			port = 6697
		}
		listener, err = tls.Listen("tcp", fmt.Sprintf(":%d", port), &config)
	} else {
		if port == 0 {
			port = 6667
		}
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
	}
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("failed to accept connection: %v", err)
		}
		go func() {
			client := ircdiscord.NewClient(conn)
			client.Debug = debug
			if err := client.Run(); err != nil {
				log.Println(err)
			}
		}()
	}
}
