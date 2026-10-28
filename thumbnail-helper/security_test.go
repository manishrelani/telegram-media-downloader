package main

import (
	"encoding/binary"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/proto"
	"testing"
)

func TestRejectsTruncatedHandshakeBeforeAllocatingPayload(t *testing.T) {
	packet := make([]byte, 20)
	binary.LittleEndian.PutUint32(packet[16:], 8*1024*1024)
	var message proto.UnencryptedMessage
	if err := message.Decode(&bin.Buffer{Buf: packet}); err == nil {
		t.Fatal("truncated handshake accepted")
	}
	if cap(message.MessageData) > 1024 {
		t.Fatalf("allocated attacker-declared payload before checking length: %d", cap(message.MessageData))
	}
}
