package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"os"
	"time"

	. "github.com/caiqfrrz/udp-file-transfer/protocol"
)

func main() {
	port := flag.String("port", "9000", "Server host port")
	flag.Parse()

	addr, err := net.ResolveUDPAddr("udp", ":"+*port)
	if err != nil {
		log.Fatalf("Error resolving address: %v", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Error listening UDP: %v", err)
	}
	defer conn.Close()

	log.Printf("Server running on port %s", *port)

	buf := make([]byte, 1500)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		h, payload, err := Unpack(buf[:n])
		if err != nil {
			log.Printf("Unpacking error: %v", err)
			continue
		}

		if h.Type == MsgTypeGet {
			filename := string(payload)
			handleGet(conn, clientAddr, filename)
		}
	}
}

func handleGet(connection *net.UDPConn, address *net.UDPAddr, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		pkt, _ := Pack(MsgTypeErr, 0, []byte("File not found"))
		connection.WriteToUDP(pkt, address)
		return
	}
	defer f.Close()

	fileInfo, err := f.Stat()
	if err == nil {
		log.Printf("Sending file %s (%d bytes) to %s", filename, fileInfo.Size(), address.String())
	}

	reader := bufio.NewReader(f)
	const payloadSize = 1024
	windowSize := 5
	timeout := 2 * time.Second

	window := make(map[uint32][]byte)
	acked := make(map[uint32]bool)
	nextSeq := uint32(0)

	for i := 0; i < windowSize; i++ {
		data := make([]byte, payloadSize)
		n, _ := io.ReadFull(reader, data)
		if n == 0 {
			break
		}

		pkt, _ := Pack(MsgTypeData, nextSeq, data[:n])
		connection.WriteToUDP(pkt, address)
		window[nextSeq] = pkt // keep for retransmition
		nextSeq++
	}

	buf := make([]byte, 1500)
	for {
		connection.SetReadDeadline(time.Now().Add(timeout))

		n, _, err := connection.ReadFromUDP(buf)
		if err != nil {
			for seq, pkt := range window {
				if !acked[seq] {
					connection.WriteToUDP(pkt, address)
				}
			}
			continue
		}

		h, _, _ := Unpack(buf[:n])

		switch h.Type {
		case MsgTypeAck:
			delete(window, h.Seq)
			acked[h.Seq] = true

			data := make([]byte, payloadSize)
			m, err := io.ReadFull(reader, data)
			if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
				log.Fatalf("error reading data: %v", err)
			}
			if m > 0 {
				pkt, _ := Pack(MsgTypeData, nextSeq, data[:m])
				connection.WriteToUDP(pkt, address)
				window[nextSeq] = pkt
				nextSeq++
			}

		case MsgTypeNak:
			if pkt, ok := window[h.Seq]; ok {
				connection.WriteToUDP(pkt, address)
			}
		}

		if len(window) == 0 {
			fin, _ := Pack(MsgTypeFin, 0, nil)
			connection.WriteToUDP(fin, address)
			connection.SetReadDeadline(time.Time{})
			return
		}
	}
}
