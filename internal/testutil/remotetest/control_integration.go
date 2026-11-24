//go:build integration

package remotetest

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/compozy/compozy/internal/remoteproto"
)

func (db *Database) SeedAccount(t *testing.T, account string) {
	t.Helper()
	db.Exec(t, "INSERT INTO %s.accounts (id, email, name, first_sign_in_provider) VALUES ($1, $2, $1, 'google')",
		account, account+"@example.test")
}

type ControlDevice struct {
	AccountID string
	ID        remoteproto.DeviceID
	JKT       string
	Access    string
	Key       *ecdsa.PrivateKey
}

func (d ControlDevice) AuthorizeRequest(t *testing.T, req *http.Request, now time.Time) {
	t.Helper()
	proof, err := remoteproto.BuildDPoP(d.Key, req.Method, remoteproto.CloudIssuer+req.URL.EscapedPath(), d.Access, now)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "DPoP "+d.Access)
	req.Header.Set("DPoP", proof)
}

func (db *Database) SeedControlDevice(
	t *testing.T, account string, id remoteproto.DeviceID, admin bool, signer *remoteproto.Signer, now time.Time,
) ControlDevice {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	jkt, err := jwk.Thumbprint()
	if err != nil {
		t.Fatal(err)
	}
	session := "ds_" + string(id)
	access, err := signer.SignDeviceAccess(remoteproto.DeviceAccessClaims{
		Issuer: remoteproto.CloudIssuer, AccountID: account, DeviceID: id, SessionID: session,
		Cnf: remoteproto.Confirmation{JKT: jkt}, JTI: rand.Text(), IssuedAt: now.Unix(),
		ExpiresAt: now.Add(remoteproto.DeviceAccessTTL).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(t, "INSERT INTO %s.devices (id, account_id, name, kind, admin) VALUES ($1, $2, $1, 'desktop', $3)",
		string(id), account, admin)
	db.Exec(t, "INSERT INTO %s.device_keys (jkt, device_id, public_jwk, state) VALUES ($1, $2, $3, 'active')",
		jkt, string(id), jwkJSON(t, jwk))
	db.Exec(t, `INSERT INTO %s.device_sessions (id, account_id, device_id, jkt, refresh_token_sha256, provider)
		VALUES ($1, $2, $3, $4, $5, 'google')`, session, account, string(id), jkt, []byte(session))
	return ControlDevice{AccountID: account, ID: id, JKT: jkt, Access: access, Key: key}
}

func (db *Database) SeedControlComputer(
	t *testing.T, account string, id remoteproto.ComputerID, linkedAt time.Time,
) ed25519.PrivateKey {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk, err := remoteproto.NewPublicJWK(public)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec(t, `INSERT INTO %s.computers
		(id, account_id, name, platform, version, computer_jwk, tls_root_sha256, remote_enabled, linked_at)
		VALUES ($1,$2,$1,'linux_x86_64','0.8.0',$3,$4,true,$5)`,
		string(id), account, jwkJSON(t, jwk), strings.Repeat("a", 64), linkedAt)
	return private
}

func (db *Database) Exec(t Fixture, statement string, args ...any) {
	t.Helper()
	if _, err := db.Pool.Exec(t.Context(), fmt.Sprintf(statement, db.Schema), args...); err != nil {
		t.Fatalf("prepare cloud fixture: %v", err)
	}
}

func jwkJSON(t *testing.T, jwk remoteproto.JWK) []byte {
	t.Helper()
	data, err := json.Marshal(jwk)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func AuthenticateControlLink(
	t *testing.T, ctx context.Context, base string, id remoteproto.ComputerID, key ed25519.PrivateKey,
) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 4 * time.Second}
	endpoint := "ws" + strings.TrimPrefix(
		base,
		"http",
	) + "/v1/computers/control?computer_id=" + url.QueryEscape(
		string(id),
	)
	link, resp, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			resp.Body.Close()
		}
		t.Fatalf("dial computer control link: %v", err)
	}
	t.Cleanup(func() {
		if err := link.Close(); err != nil {
			t.Logf("control socket closed: %v", err)
		}
	})
	message := ReadControlMessage(t, link, 5*time.Second)
	var challenge remoteproto.ComputerChallenge
	if message.Type != remoteproto.ControlChallenge || json.Unmarshal(message.Data, &challenge) != nil ||
		challenge.Nonce == "" {
		t.Fatalf("expected a computer challenge, got %s", message.Type)
	}
	SendControl(t, link, remoteproto.ControlAuthenticate, remoteproto.ComputerAuthentication{
		Signature: remoteproto.SignComputerChallenge(key, remoteproto.CloudIssuer, id, challenge.Nonce),
	})
	return link
}

func SendControl(t *testing.T, link *websocket.Conn, kind remoteproto.ControlType, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := link.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := link.WriteJSON(remoteproto.ControlMessage{Type: kind, Data: data}); err != nil {
		t.Fatalf("send control %s: %v", kind, err)
	}
}

func ReadControlMessage(t *testing.T, link *websocket.Conn, limit time.Duration) remoteproto.ControlMessage {
	t.Helper()
	if err := link.SetReadDeadline(time.Now().Add(limit)); err != nil {
		t.Fatal(err)
	}
	_, raw, err := link.ReadMessage()
	if err != nil {
		t.Fatalf("read control frame: %v", err)
	}
	var message remoteproto.ControlMessage
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatalf("decode control frame: %v", err)
	}
	return message
}
