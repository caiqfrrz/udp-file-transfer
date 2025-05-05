package main

import (
	"log"
	"net"
)

const (
	MsgTypeGet  = 0x01
	MsgTypeData = 0x02
	MsgTypeAck  = 0x03
	MsgTypeNak  = 0x04
	MsgTypeErr  = 0x05
	MsgTypeFin  = 0x06
)

type Header struct {
	Type     byte   // 1 byte
	Seq      uint32 // 4 bytes
	Length   uint16 // 2 bytes
	Checksum uint32 // 4 bytes
}

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
