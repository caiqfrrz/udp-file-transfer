package main

import (
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"math/rand"
	"os"
	"strings"
	"syscall"

	. "github.com/caiqfrrz/udp-file-transfer/protocol"
)

var debug = false

func debugLog(format string, v ...interface{}) {
	if debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func setDebug(set bool) {
	debug = set
}

func main() {
	server := flag.String("server", "127.0.0.1:9000", "UDP server address")
	file := flag.String("file", "test.dat", "Name of file to request")
	drop := flag.Bool("drop", false, "Simulate package dropping")
	debug := flag.Bool("debug", false, "Debug mode log")
	flag.Parse()

	setDebug(*debug)
	if err := requestFile(*server, *file, *drop); err != nil {
		log.Fatalf("transfer failed: %v", err)
	}
	log.Printf("File %q received successfully!", *file)
}

func requestFile(server string, filename string, drop bool) error {
	ipStr, portStr, err := splitHostPort(server)
	if err != nil {
		return fmt.Errorf("invalid server address: %v", err)
	}

	port := Atoi(portStr)

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	debugLog("socket created")
	if err != nil {
		return fmt.Errorf("socket creation failed: %v", err)
	}
	defer syscall.Close(fd)

	var sa syscall.SockaddrInet4
	sa.Port = port
	if err := ipToBytes(ipStr, sa.Addr[:]); err != nil {
		return fmt.Errorf("invalid IP: %v", err)
	}

	// send GET
	getPkt, _ := Pack(MsgTypeGet, 0, []byte(filename))
	if err := syscall.Sendto(fd, getPkt, 0, &sa); err != nil {
		return fmt.Errorf("sendto failed: %v", err)
	}
	debugLog("get sent to server")

	received := make(map[uint32][]byte)
	buf := make([]byte, 1500)

	for {
		tv := syscall.Timeval{Sec: 5, Usec: 0}
		if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
			return fmt.Errorf("timeout setting error: %v", err)
		}

		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				return fmt.Errorf("timeout: no packet received in 5s, system may be down")
			}
			return fmt.Errorf("recvfrom failed: %v", err)
		}

		h, payload, err := Unpack(buf[:n])
		if err != nil {
			return fmt.Errorf("unpack failed: %v", err)
		}

		switch h.Type {
		case MsgTypeData:
			debugLog("received payload seq: %d", h.Seq)
			if drop && rand.Float64() > 0.99 {
				simulateCorruption(payload)
			}
			//validate checksum
			if crc32.ChecksumIEEE(payload) != h.Checksum {
				log.Printf("Checksum mismatch for Seq=%d", h.Seq)
				nak, _ := Pack(MsgTypeNak, h.Seq, nil)
				if err := syscall.Sendto(fd, nak, 0, &sa); err != nil {
					return fmt.Errorf("sendto NAK failed: %v", err)
				}
			} else {
				received[h.Seq] = append([]byte(nil), payload...)
				ack, _ := Pack(MsgTypeAck, h.Seq, nil)
				if err := syscall.Sendto(fd, ack, 0, &sa); err != nil {
					return fmt.Errorf("sendto ACK failed: %v", err)
				}
			}
		case MsgTypeErr:
			return fmt.Errorf("server error: %s", payload)

		case MsgTypeFin:
			//assemble file
			return assembleFile(filename, received)
		}
	}
}

func splitHostPort(hostport string) (host, port string, err error) {
	parts := strings.Split(hostport, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid host:port format")
	}
	return parts[0], parts[1], nil
}

func ipToBytes(ipStr string, b []byte) error {
	parts := strings.Split(ipStr, ".")
	if len(parts) != 4 {
		return fmt.Errorf("invalid IPv4 address")
	}

	for i, part := range parts {
		num := Atoi(part)
		if num < 0 || num > 255 {
			return fmt.Errorf("invalid IP byte")
		}
		b[i] = byte(num)
	}
	return nil
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
