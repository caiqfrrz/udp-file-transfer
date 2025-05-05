package main

import (
	"log"
	"net"
)

func main() {
	serverAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:9000")
	if err != nil {
		log.Fatalf("Error solving server address: %v", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatalf("Error creating UDP connection: %v", err)
	}
	defer conn.Close()

	conn.Write([]byte("Ola servidor"))
	buf := make([]byte, 1024)
	n, _, _ := conn.ReadFromUDP(buf)
	log.Printf("Resposta: %s\n", string(buf[:n]))
}
