// SPDX-License-Identifier: FSL-1.1-MIT

package email

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

const UnsubscribePath = "/v1/email/unsubscribe"

type UnsubscribeLinks struct {
	origin string
	key    []byte
}

func NewUnsubscribeLinks(origin string, key []byte) (*UnsubscribeLinks, error) {
	if len(key) != 32 {
		return nil, errors.New("unsubscribe signing requires a 32-byte key")
	}
	return &UnsubscribeLinks{origin: origin, key: bytes.Clone(key)}, nil
}

func (s *UnsubscribeLinks) URL(accountID string) string {
	return s.origin + UnsubscribePath + "?" + url.Values{"t": {accountID + "." + s.signature(accountID)}}.Encode()
}

func (s *UnsubscribeLinks) AccountID(token string) (string, bool) {
	id, signature, ok := strings.Cut(token, ".")
	if !ok || !strings.HasPrefix(id, "acc_") || len(id) > 128 || len(signature) != 43 {
		return "", false
	}
	return id, hmac.Equal([]byte(signature), []byte(s.signature(id)))
}

func (s *UnsubscribeLinks) signature(id string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte("compozy-email-unsubscribe-v1\n" + id))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
