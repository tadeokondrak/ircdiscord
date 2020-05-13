package main

import (
	"flag"
	"log"
	"net"

	ircdiscord "github.com/tadeokondrak/ircdiscord/src"
)

func main() {
	var (
		debug bool
	)
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()

	log.SetFlags(0)
	listener, err := net.Listen("tcp", ":6667")
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
