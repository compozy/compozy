package remotehost

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"sync"
	"time"
)

const (
	computerKeyRef  = "computer-key"
	tlsRootRef      = "tls-root"
	leafTTL         = 90 * 24 * time.Hour
	leafRenewBefore = 30 * 24 * time.Hour
)

type KeyStore interface {
	GetHostSecret(string) (string, bool, error)
	PutHostSecret(string, string) error
	DeleteHostSecret(string) error
}

type hostKeys struct {
	signing ed25519.PrivateKey
	ca      *certificateAuthority
}

type certificateAuthority struct {
	store   KeyStore
	ref     string
	root    *x509.Certificate
	key     *ecdsa.PrivateKey
	mu      sync.Mutex
	leaf    *tls.Certificate
	retired bool
}

type authorityMaterial struct {
	Root    []byte `json:"root"`
	RootKey []byte `json:"root_key"`
	Leaf    []byte `json:"leaf"`
	LeafKey []byte `json:"leaf_key"`
}

func loadHostKeys(store KeyStore, identity Identity, now time.Time) (hostKeys, error) {
	create := identity.ComputerID == ""
	keyRef, rootRef := identity.ComputerKeyRef, identity.TLSRootRef
	if create {
		keyRef, rootRef = computerKeyRef, tlsRootRef
	}
	signing, err := loadSigningKey(store, keyRef, create)
	if err != nil {
		return hostKeys{}, err
	}
	authority, err := loadAuthority(store, rootRef, create, now)
	if err != nil {
		return hostKeys{}, err
	}
	if !create && identity.TLSRootSHA256 != authority.fingerprint() {
		return hostKeys{}, errors.New("computer TLS root does not match its linked identity")
	}
	return hostKeys{signing: signing, ca: authority}, nil
}

func loadSigningKey(store KeyStore, ref string, create bool) (ed25519.PrivateKey, error) {
	raw, present, err := store.GetHostSecret(ref)
	if err != nil {
		return nil, err
	}
	if present {
		seed, err := base64.RawStdEncoding.DecodeString(raw)
		if err != nil || len(seed) != ed25519.SeedSize {
			return nil, errors.New("invalid computer signing key")
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if !create {
		return nil, errors.New("computer signing key is missing")
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	if err := store.PutHostSecret(ref, base64.RawStdEncoding.EncodeToString(private.Seed())); err != nil {
		return nil, err
	}
	return private, nil
}

func loadAuthority(store KeyStore, ref string, create bool, now time.Time) (*certificateAuthority, error) {
	raw, present, err := store.GetHostSecret(ref)
	if err != nil {
		return nil, err
	}
	if present {
		return parseAuthority(store, ref, raw)
	}
	if !create {
		return nil, errors.New("computer TLS root is missing")
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	root := &x509.Certificate{
		Subject: pkix.Name{CommonName: "CompozyOS Computer Root"}, NotBefore: now.Add(-time.Minute),
		NotAfter: now.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, MaxPathLenZero: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, root, root, private.Public(), private)
	if err != nil {
		return nil, err
	}
	root, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	authority := &certificateAuthority{store: store, ref: ref, root: root, key: private}
	_, err = authority.certificate(now)
	return authority, err
}

func parseAuthority(store KeyStore, ref, raw string) (*certificateAuthority, error) {
	var material authorityMaterial
	if err := json.Unmarshal([]byte(raw), &material); err != nil {
		return nil, errors.New("invalid computer certificate material")
	}
	root, err := x509.ParseCertificate(material.Root)
	if err != nil || !root.IsCA || root.CheckSignatureFrom(root) != nil {
		return nil, errors.New("invalid computer TLS root")
	}
	key, err := x509.ParseECPrivateKey(material.RootKey)
	if err != nil || key.Curve != elliptic.P256() || !key.PublicKey.Equal(root.PublicKey) {
		return nil, errors.New("invalid computer TLS root key")
	}
	leaf, err := x509.ParseCertificate(material.Leaf)
	if err != nil || leaf.CheckSignatureFrom(root) != nil {
		return nil, errors.New("invalid computer TLS leaf certificate")
	}
	leafKey, err := x509.ParseECPrivateKey(material.LeafKey)
	if err != nil || leafKey.Curve != elliptic.P256() || !leafKey.PublicKey.Equal(leaf.PublicKey) {
		return nil, errors.New("invalid computer TLS leaf key")
	}
	return &certificateAuthority{store: store, ref: ref, root: root, key: key, leaf: &tls.Certificate{
		Certificate: [][]byte{leaf.Raw, root.Raw}, PrivateKey: leafKey, Leaf: leaf,
	}}, nil
}

func (a *certificateAuthority) fingerprint() string {
	sum := sha256.Sum256(a.root.Raw)
	return hex.EncodeToString(sum[:])
}

func (a *certificateAuthority) certificate(now time.Time) (*tls.Certificate, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.retired {
		return nil, ErrNotLinked
	}
	if a.leaf != nil && now.Add(leafRenewBefore).Before(a.leaf.Leaf.NotAfter) {
		return a.leaf, nil
	}
	if !now.Before(a.root.NotAfter) {
		return nil, errors.New("computer TLS root expired")
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	// The certificate carries the computer identity through its pinned root.
	leaf := &x509.Certificate{
		Subject: pkix.Name{CommonName: "CompozyOS Computer"}, DNSNames: []string{"compozy-remote"},
		NotBefore: now.Add(-time.Minute), NotAfter: minTime(now.Add(leafTTL), a.root.NotAfter),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, leaf, a.root, key.Public(), a.key)
	if err != nil {
		return nil, err
	}
	leaf, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	if err := a.save(der, key); err != nil {
		return nil, err
	}
	a.leaf = &tls.Certificate{Certificate: [][]byte{der, a.root.Raw}, PrivateKey: key, Leaf: leaf}
	return a.leaf, nil
}

func (a *certificateAuthority) retire() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.retired = true
}

func (a *certificateAuthority) save(leaf []byte, key *ecdsa.PrivateKey) error {
	rootKey, err := x509.MarshalECPrivateKey(a.key)
	if err != nil {
		return err
	}
	leafKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	material, err := json.Marshal(authorityMaterial{a.root.Raw, rootKey, leaf, leafKey})
	if err != nil {
		return err
	}
	return a.store.PutHostSecret(a.ref, string(material))
}

func (a *certificateAuthority) serverTLS(
	now func() time.Time,
	renew func(context.Context, time.Time) error,
) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := a.certificate(now())
			if err != nil {
				return nil, err
			}
			if err := renew(hello.Context(), cert.Leaf.NotAfter); err != nil {
				return nil, err
			}
			return cert, nil
		},
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
