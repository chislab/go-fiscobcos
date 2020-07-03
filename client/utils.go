package client

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"strings"
)

func newSeq() string {
	var buf [16]byte
	io.ReadFull(rand.Reader, buf[:])
	return hex.EncodeToString(buf[:])
}

func NewFilterID() string {
	return newSeq()
}

func getReceiptOutput(output string) string {
	if strings.HasPrefix(output, "0x") {
		output = output[2:]
	}
	b, err := hex.DecodeString(output)
	if err != nil || len(b) < 36 {
		return output
	}
	b = b[36:]
	tail := len(b) - 1
	for ; tail >= 0; tail-- {
		if b[tail] != 0 {
			break
		}
	}
	return string(b[:tail+1])
}
