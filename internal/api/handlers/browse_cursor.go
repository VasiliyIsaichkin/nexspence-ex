package handlers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
)

type browseCursor struct {
	After string
	ID    string
}

func newBrowseCipher() cipher.AEAD {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return aead
}
func (h *BrowseHandler) encodeCursor(value browseCursor, binding string) string {
	data, _ := json.Marshal(value)
	nonce := make([]byte, h.cursor.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(h.cursor.Seal(nonce, nonce, data, []byte(binding)))
}
func (h *BrowseHandler) decodeCursor(raw, binding string) (browseCursor, error) {
	var result browseCursor
	if raw == "" {
		return result, nil
	}
	if len(raw) > 4096 {
		return result, errors.New("invalid cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(raw)
	n := h.cursor.NonceSize()
	if err != nil || len(data) < n {
		return result, errors.New("invalid cursor")
	}
	data, err = h.cursor.Open(nil, data[:n], data[n:], []byte(binding))
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}
