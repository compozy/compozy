// SPDX-License-Identifier: FSL-1.1-MIT

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

func (k *SigningKeys) Filtered(retired []string) (remoteproto.JWKS, error) {
	if slices.Contains(retired, k.active.PublicJWK().Kid) {
		return remoteproto.JWKS{}, errors.New("the configured cloud signer was retired")
	}
	keys := k.JWKS()
	keys.Keys = slices.DeleteFunc(keys.Keys, func(key remoteproto.JWK) bool {
		return slices.Contains(retired, key.Kid)
	})
	keys.Retired = retired
	return keys, nil
}
