# UDP Reliable file transfer

This project implements a **simple reliable file transfer** application over UDP in Go, demonstrating how to build basic reliability features (segmentation, checksums, acknowledgments, retransmissions) on top of an unreliable transport.

## Features

- **Custom Aplication Protocol**: Fixed header per datagram containing message type, sequence number, payload length and CRC32 checksum.
- **Sliding Window**: Configurable sliding window size for pipelined transmission.
- **Selective Retransmission**: Uses ACK/NAK for individual segments with timeout-based fallback.
- **Minimal Dependencies**: Uses Go's standard `net` package, without high level UDP abstractions.

## Protocol Details

- 1 byte Message Type (GET, DATA, ACK, NAK, FIN, ERR)
- 4 bytes Sequence Number (uint32, big-endian)
- 2 bytes Payload Length (uint16, big-endian)
- 4 bytes CRC32 Checksum (uint32, big-endian)
- N bytes Payload

### Message Types

| Type | Hex  | Description                          |
| ---- | ---- | ------------------------------------ |
| GET  | 0x01 | Client requests a filename           |
| DATA | 0x02 | Server sends a data segment          |
| ACK  | 0x03 | Client acknowledges a segment        |
| NAK  | 0x04 | Client requests re-send of a segment |
| FIN  | 0x05 | Server signals end-of-transfer       |
| ERR  | 0x06 | Server signals an error              |

## Repository Layout

```bash
udp-file-transfer/
├── go.mod
├── protocol/ # shared Pack/Unpack and message definitions
│       └── packing.go # Pack, Unpack, MsgType constants
├── cmd/
│ ├── server/
│ │     └── main.go # UDP server
│ └── client/
│       └── main.go # UDP client
└── README.md # this file
```

## Prerequisites

- Go 1.18 or higher

## Building

```bash
# From project root
go mod tidy
go build -o bin/server ./cmd/server
go build -o bin/client ./cmd/client
```

## Usage

### Starting the server

```bash
# Serve files from the current directory on UDP port 9000
./bin/server -port 9000
```

### Downloading a file with the client

```bash
# Generate a test file and request it
dd if=/dev/zero bs=1M count=2 of=sample.log   # create ~2 MiB test file
./bin/client -server 127.0.0.1:9000 -file sample.log
```

On successful transfer, the client writes `recreated_sample.log` in its working directory.
