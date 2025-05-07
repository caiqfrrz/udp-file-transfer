package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"syscall"
	"time"

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
	port := flag.String("port", "9000", "Server host port")
	debug := flag.Bool("debug", false, "Debug logs")
	flag.Parse()

	setDebug(*debug)
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		log.Fatalf("Socket creation failed: %v", err)
	}
	defer syscall.Close(fd)
	debugLog("socket created")

	addr := syscall.SockaddrInet4{Port: Atoi(*port)}
	if err := syscall.Bind(fd, &addr); err != nil {
		log.Fatalf("bind failed: %v", err)
	}

	log.Printf("Server running on port %s", *port)

	buf := make([]byte, 1500)
	for {
		tv := syscall.Timeval{Sec: 30, Usec: 0}
		syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)

		n, clientSA, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}

			if err != syscall.EAGAIN && err != syscall.EWOULDBLOCK {
				log.Printf("Recvfrom failed: %v", err)
			}

			time.Sleep(100 * time.Millisecond)
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

	retransmits := make(map[uint32]int)
	maxRetransmits := 5

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
	debugLog("window is full")

	tv := syscall.Timeval{Sec: 2, Usec: 0}
	if err := syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv); err != nil {
		log.Printf("Failed to set socket timeout: %v", err)
		return
	}

	// wait for ACK/NAK
	buf := make([]byte, 1500)
	for {
		n, _, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}

			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK {
				for seq, pkt := range window {
					retransmits[seq]++
					if retransmits[seq] > maxRetransmits {
						log.Printf("maximum retransmits reached for packet %d, giving up", seq)
						fin, _ := Pack(MsgTypeFin, 0, []byte("Maximum retransmits exceeded"))
						syscall.Sendto(fd, fin, 0, address)
						return
					}

					log.Printf("Timeout: Resending packet %d (attempt %d/%d)",
						seq, retransmits[seq], maxRetransmits)
					syscall.Sendto(fd, pkt, 0, address)
				}
				continue
			}
			log.Printf("recvfrom failed: %v", err)
			continue
		}

		h, _, _ := Unpack(buf[:n])

		if h.Type == MsgTypeAck {
			retransmits[h.Seq] = 0
		}

		switch h.Type {
		case MsgTypeAck:
			debugLog("ack for packet seq: %d", h.Seq)
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
			debugLog("nak for packet seq: %d", h.Seq)
			if pkt, ok := window[h.Seq]; ok {
				log.Printf("Resending packet of sequence %d", h.Seq)
				syscall.Sendto(fd, pkt, 0, address)
			}
		}

		if len(window) == 0 {
			debugLog("end")
			fin, _ := Pack(MsgTypeFin, 0, nil)
			syscall.Sendto(fd, fin, 0, address)
			return
		}
	}
}
