package api

import (
	"errors"
	"slices"

	"github.com/compozy/compozy/internal/remoteproto"
)

type SigningKeys struct {
	active *remoteproto.Signer
	public remoteproto.JWKS
}

func NewSigningKeys(active *remoteproto.Signer, overlap []remoteproto.JWK) (*SigningKeys, error) {
	if active == nil {
		return nil, errors.New("an active cloud signing key is required")
	}
	keySet := remoteproto.JWKS{Keys: append([]remoteproto.JWK{active.PublicJWK()}, overlap...)}
	if _, err := remoteproto.NewJWSVerifier(remoteproto.CloudIssuer, keySet, nil); err != nil {
		return nil, err
	}
	return &SigningKeys{active: active, public: keySet}, nil
}

func (k *SigningKeys) Signer() *remoteproto.Signer {
	return k.active
}

func (k *SigningKeys) JWKS() remoteproto.JWKS {
	return remoteproto.JWKS{Keys: slices.Clone(k.public.Keys)}
}
