package channel

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

func newSeq() string {
	var buf [16]byte
	io.ReadFull(rand.Reader, buf[:])
	return hex.EncodeToString(buf[:])
}

func NewFilterID() string {
	return newSeq()
}
