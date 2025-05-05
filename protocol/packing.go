package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

type Header struct {
	Type     byte   // 1 byte
	Seq      uint32 // 4 bytes
	Length   uint16 // 2 bytes
	Checksum uint32 // 4 bytes
}

const (
	MsgTypeGet  = 0x01
	MsgTypeData = 0x02
	MsgTypeAck  = 0x03
	MsgTypeNak  = 0x04
	MsgTypeErr  = 0x05
	MsgTypeFin  = 0x06
)

func Pack(msgType byte, msgSeq uint32, data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(nil)

	// header specific bytes
	buf.WriteByte(msgType)
	binary.Write(buf, binary.BigEndian, msgSeq)
	binary.Write(buf, binary.BigEndian, uint16(len(data)))
	cs := crc32.ChecksumIEEE(data)
	binary.Write(buf, binary.BigEndian, cs)

	// payload
	buf.Write(data)

	return buf.Bytes(), nil
}

func Unpack(packet []byte) (Header, []byte, error) {
	var h Header
	if len(packet) < 11 {
		return h, nil, fmt.Errorf("packet too short")
	}

	buf := bytes.NewReader(packet)

	binary.Read(buf, binary.BigEndian, &h.Type)
	binary.Read(buf, binary.BigEndian, &h.Seq)
	binary.Read(buf, binary.BigEndian, &h.Length)
	binary.Read(buf, binary.BigEndian, &h.Checksum)

	payload := packet[11 : 11+int(h.Length)]
	return h, payload, nil
}
