package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	encryptedPrefix    = "aes-gcm-v1:"
	ciphertextAADLabel = "compozy:vault:aes-gcm:v1"
)

type ciphertextIdentity struct {
	ref  string
	kind string
}

func newCiphertextIdentity(ref string, kind string) (ciphertextIdentity, error) {
	normalizedRef := NormalizeRef(ref)
	if err := ValidateSecretRef(normalizedRef); err != nil {
		return ciphertextIdentity{}, fmt.Errorf("vault: bind ciphertext identity: %w", err)
	}
	return ciphertextIdentity{ref: normalizedRef, kind: strings.TrimSpace(kind)}, nil
}

func (i ciphertextIdentity) additionalData() []byte {
	data := make([]byte, 0, len(ciphertextAADLabel)+len(i.ref)+len(i.kind)+20)
	data = append(data, ciphertextAADLabel...)
	data = binary.AppendUvarint(data, uint64(len(i.ref)))
	data = append(data, i.ref...)
	data = binary.AppendUvarint(data, uint64(len(i.kind)))
	return append(data, i.kind...)
}

func encryptValue(key []byte, plaintext string, identity ciphertextIdentity) (string, error) {
	if len(key) != keySizeBytes {
		return "", fmt.Errorf("vault: key must be %d bytes", keySizeBytes)
	}
	if strings.TrimSpace(plaintext) == "" {
		return "", ErrMissingSecret
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("vault: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("vault: create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("vault: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), identity.additionalData())
	payload := make([]byte, 0, len(nonce)+len(ciphertext))
	payload = append(payload, nonce...)
	payload = append(payload, ciphertext...)
	return encryptedPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func decryptValue(key []byte, encrypted string, identity ciphertextIdentity) (string, error) {
	if len(key) != keySizeBytes {
		return "", fmt.Errorf("vault: key must be %d bytes", keySizeBytes)
	}
	if !strings.HasPrefix(encrypted, encryptedPrefix) {
		return "", errors.New("vault: encrypted value is missing aes-gcm-v1 prefix")
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encrypted, encryptedPrefix))
	if err != nil {
		return "", fmt.Errorf("vault: decode encrypted value: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("vault: create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("vault: create gcm: %w", err)
	}
	if len(payload) <= gcm.NonceSize() {
		return "", errors.New("vault: encrypted value is truncated")
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, identity.additionalData())
	if err != nil {
		return "", fmt.Errorf("vault: decrypt value: %w", err)
	}
	return string(plaintext), nil
}
