package providerusage

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"log/slog"
)

var identityKey = func() [32]byte {
	var key [32]byte
	rand.Read(key[:])
	return key
}()

func NewCredentialIdentity(source SourceKind, credential []byte, accountID string) CredentialIdentity {
	h := hmac.New(sha256.New, identityKey[:])
	h.Write([]byte(source))
	h.Write(credential)
	return CredentialIdentity{Digest: [32]byte(h.Sum(nil)), AccountID: accountID}
}

func (a CredentialIdentity) Same(b CredentialIdentity) bool {
	if a.AccountID != "" && b.AccountID != "" {
		return a.AccountID == b.AccountID
	}
	return hmac.Equal(a.Digest[:], b.Digest[:])
}

var _ fmt.Formatter = CredentialIdentity{}
var _ slog.LogValuer = CredentialIdentity{}

func (CredentialIdentity) Format(s fmt.State, _ rune) {
	fmt.Fprint(s, "[redacted credential identity]")
}
func (CredentialIdentity) LogValue() slog.Value {
	return slog.StringValue("[redacted credential identity]")
}
