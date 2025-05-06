package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"syscall"

	. "github.com/caiqfrrz/udp-file-transfer/protocol"
)

func main() {
	port := flag.String("port", "9000", "Server host port")
	flag.Parse()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		log.Fatalf("Socket creation failed: %v", err)
	}
	defer syscall.Close(fd)

	addr := syscall.SockaddrInet4{Port: atoi(*port)}
	if err := syscall.Bind(fd, &addr); err != nil {
		log.Fatalf("bind failed: %v", err)
	}

	log.Printf("Server running on port %s", *port)

	buf := make([]byte, 1500)
	for {
		n, clientSA, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			log.Printf("Recvfrom failed: %v", err)
			continue
		}

		h, payload, err := Unpack(buf[:n])
		if err != nil {
			log.Printf("Unpacking error: %v", err)
			continue
		}

		if h.Type == MsgTypeGet {
			filename := string(payload)
			handleGet(fd, clientSA.(*syscall.SockaddrInet4), filename)
		}
	}
}

func handleGet(fd int, address *syscall.SockaddrInet4, filename string) {
	f, err := os.Open(filename)
	if err != nil {
		pkt, _ := Pack(MsgTypeErr, 0, []byte("File not found"))
		syscall.Sendto(fd, pkt, 0, address)
		return
	}
	defer f.Close()

	// Log file info (optional)
	if stat, err := f.Stat(); err == nil {
		log.Printf("Sending %s (%d bytes) to %d.%d.%d.%d:%d",
			filename, stat.Size(),
			address.Addr[0], address.Addr[1], address.Addr[2], address.Addr[3],
			address.Port,
		)
	}

	reader := bufio.NewReader(f)
	const (
		payloadSize = 1024
		windowSize  = 5
	)

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
		syscall.Sendto(fd, pkt, 0, address)
		window[nextSeq] = pkt // keep for retransmition
		nextSeq++
	}

	// wait for ACK/NAK
	buf := make([]byte, 1500)
	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			log.Printf("recvfrom failed: %v", err)
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
				syscall.Sendto(fd, pkt, 0, address)
				window[nextSeq] = pkt
				nextSeq++
			}

		case MsgTypeNak:
			if pkt, ok := window[h.Seq]; ok {
				log.Printf("Resending packet of sequence %d", h.Seq)
				syscall.Sendto(fd, pkt, 0, address)
			}
		}

		if len(window) == 0 {
			fin, _ := Pack(MsgTypeFin, 0, nil)
			syscall.Sendto(fd, fin, 0, address)
			return
		}
	}
}

func syscallSendTo(fd int, pkt []byte, addr *net.UDPAddr) error {
	sa := &syscall.SockaddrInet4{
		Port: addr.Port,
	}
	copy(sa.Addr[:], addr.IP.To4())
	return syscall.Sendto(fd, pkt, 0, sa)
}

func sockaddrToUDPAddr(sa syscall.Sockaddr) *net.UDPAddr {
	switch addr := sa.(type) {
	case *syscall.SockaddrInet4:
		return &net.UDPAddr{
			IP:   net.IPv4(addr.Addr[0], addr.Addr[1], addr.Addr[2], addr.Addr[3]),
			Port: addr.Port,
		}
	default:
		return nil
	}
}

func atoi(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
