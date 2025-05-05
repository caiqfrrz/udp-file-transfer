package main

import (
	"log"
	"net"
)

func main() {
	addr, err := net.ResolveUDPAddr("udp", ":9000")
	if err != nil {
		log.Fatalf("Error resolving address: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Error listening UDP: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, clientAddr, _ := conn.ReadFromUDP(buf)
		log.Printf("Recebido: %s\n", string(buf[:n]))
		conn.WriteToUDP([]byte("Olá do servidor!"), clientAddr)
	}
}
