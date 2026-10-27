package id

import (
	"crypto/rand"
	"encoding/hex"
)

func New(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)

	if prefix == "" {
		return hex.EncodeToString(buf)
	}
	return prefix + "_" + hex.EncodeToString(buf)
}
