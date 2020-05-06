package main

import (
	"log"
	"net"

	ircdiscord "github.com/tadeokondrak/ircdiscord/src"
)

func main() {
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
			if err := client.Run(); err != nil {
				log.Println(err)
			}
		}()
	}
}
