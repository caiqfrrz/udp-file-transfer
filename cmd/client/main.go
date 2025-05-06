package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	. "github.com/caiqfrrz/udp-file-transfer/protocol"
)

func main() {
	server := flag.String("server", "127.0.0.1:9000", "UDP server address")
	file := flag.String("file", "test.dat", "Name of file to request")
	drop := flag.Bool("drop", false, "Simulate package dropping")
	flag.Parse()

	if err := requestFile(*server, *file, *drop); err != nil {
		log.Fatalf("transfer failed: %v", err)
	}
	log.Printf("File %q received successfully!", *file)
}

func requestFile(server string, filename string, drop bool) error {
	addr, _ := net.ResolveUDPAddr("udp", server)
	conn, _ := net.DialUDP("udp", nil, addr)
	defer conn.Close()

	// send GET
	getPkt, _ := Pack(MsgTypeGet, 0, []byte(filename))
	conn.Write(getPkt)

	received := make(map[uint32][]byte)
	buf := make([]byte, 1500)
	timeout := 5 * time.Second

	for {
		conn.SetReadDeadline(time.Now().Add(timeout))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return fmt.Errorf("timeout or reading error: %v", err)
		}
		h, payload, _ := Unpack(buf[:n])

		switch h.Type {
		case MsgTypeData:
			if rand.Float64() > 0.99 {
				simulateCorruption(payload)
			}
			//validate checksum
			if crc32.ChecksumIEEE(payload) != h.Checksum {
				log.Printf("Checksum mismatch for Seq=%d", h.Seq)
				nak, _ := Pack(MsgTypeNak, h.Seq, nil)
				conn.Write(nak)
			} else {
				received[h.Seq] = append([]byte(nil), payload...)
				ack, _ := Pack(MsgTypeAck, h.Seq, nil)
				conn.Write(ack)
			}
		case MsgTypeErr:
			return fmt.Errorf("server error: %s", payload)

		case MsgTypeFin:
			//assemble file
			return assembleFile(filename, received)
		}
	}
}

func assembleFile(fileName string, chunks map[uint32][]byte) error {
	out, err := os.Create("recreated_" + fileName)
	if err != nil {
		return fmt.Errorf("error reconstructing file: %v", err)
	}
	defer out.Close()

	for seq := uint32(0); ; seq++ {
		data, ok := chunks[seq]
		if !ok {
			log.Printf("Missing chunk: Seq=%d", seq)
			break
		}
		out.Write(data)
	}
	return nil
}

func simulateCorruption(payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("empty payload")
	}
	for i := 0; i < len(payload)/2; i++ {
		payload[rand.Intn(len(payload))] ^= 0xFF
	}

	return nil
}
