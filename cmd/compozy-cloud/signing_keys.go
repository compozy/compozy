// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"io"
	"regexp"
	"time"

	"github.com/compozy/compozy/internal/cloud/store"
)

var signingKeyID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func generateSigningKey(kid string, out io.Writer) error {
	if !signingKeyID.MatchString(kid) {
		return errors.New("invalid signing key ID")
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	return json.MarshalWrite(
		out,
		cloudSigningConfig{KID: kid, Seed: base64.RawURLEncoding.EncodeToString(private.Seed())},
	)
}

func printSigningJWK(getenv func(string) string, out io.Writer) error {
	keys, err := loadSigningKeys(getenv)
	if err != nil {
		return err
	}
	return json.MarshalWrite(out, keys.Signer().PublicJWK())
}

func publishSigningKeys(ctx context.Context, retired string, getenv func(string) string) error {
	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return err
	}
	direct, err := databaseURL(getenv, envNeonDirect, environment)
	if err != nil {
		return err
	}
	if retired != "" {
		keys, err := loadSigningKeys(getenv)
		if err != nil {
			return err
		}
		if !signingKeyID.MatchString(retired) || keys.Signer().PublicJWK().Kid == retired {
			return errors.New("retirement requires a valid key ID distinct from the active signer")
		}
	}
	db, err := store.Open(ctx, store.Config{PooledDSN: direct, DirectDSN: direct})
	if err != nil {
		return err
	}
	defer db.Close()
	return db.PublishSigningKeys(ctx, retired, time.Now())
}
