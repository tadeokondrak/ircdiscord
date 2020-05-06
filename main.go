package main

import (
	"log"
	"net"
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
		client := NewClient(conn)
		go func() {
			if err := client.Run(); err != nil {
				log.Println(err)
			}
		}()
	}
}
