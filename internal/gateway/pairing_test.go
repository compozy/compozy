package gateway

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDeviceCredentialsAndPairing(t *testing.T) {
	t.Parallel()

	t.Run("Should issue a 256-bit prefixed credential and persist only its hash (UT-030)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, store := newDeviceServiceFixture(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "Laptop", ActorKindCLIProfile)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		encoded := strings.TrimPrefix(issued.Credential, deviceCredentialPrefix)
		decoded, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(decoded) != secretEntropyBytes {
			t.Fatalf("credential entropy = %d bytes, error = %v", len(decoded), err)
		}
		record := store.byID[issued.Device.ID]
		if record.TokenHash != hashCredential(issued.Credential) || record.TokenHash == issued.Credential ||
			len(record.TokenHash) != 64 {
			t.Fatalf("stored credential = %#v, want SHA-256 hex only", record)
		}
	})

	t.Run("Should activate a CLI-provisioned credential without persisting its raw value", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, store := newDeviceServiceFixture(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		credential, err := GenerateDeviceCredential()
		if err != nil {
			t.Fatalf("GenerateDeviceCredential() error = %v", err)
		}
		issued, err := service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: artifact.Artifact, Name: "CLI", Kind: ActorKindCLIProfile,
			Source: PairingSourcePrivate, Credential: credential,
		})
		if err != nil {
			t.Fatalf("RedeemPairing(client credential) error = %v", err)
		}
		if issued.Credential != credential {
			t.Fatalf("issued credential differs from the client-provisioned credential")
		}
		record := store.byID[issued.Device.ID]
		if record.TokenHash != hashCredential(credential) || record.TokenHash == credential {
			t.Fatalf("stored credential = %#v, want only the client credential hash", record)
		}
		if _, err := service.Authenticate(t.Context(), credential); err != nil {
			t.Fatalf("Authenticate(client credential) error = %v", err)
		}
	})

	t.Run("Should authenticate through the same constant-time comparison path (UT-031)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		issued, err := issueTestDevice(t.Context(), service, "CLI", ActorKindCLIProfile)
		if err != nil {
			t.Fatalf("issueTestDevice() error = %v", err)
		}
		got, err := service.Authenticate(t.Context(), issued.Credential)
		if err != nil || got.ID != issued.Device.ID {
			t.Fatalf("Authenticate(valid) = %#v, %v", got, err)
		}
		wrong := issued.Credential[:len(issued.Credential)-1] + "A"
		if _, err := service.Authenticate(t.Context(), wrong); !errors.Is(err, ErrDeviceUnauthenticated) {
			t.Fatalf("Authenticate(wrong) error = %v", err)
		}
		if constantTimeCredentialMatch(wrong, hashCredential(issued.Credential)) {
			t.Fatal("constantTimeCredentialMatch(wrong) = true")
		}
	})

	t.Run("Should reject an expired artifact without creating a session (UT-032)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, store := newDeviceServiceFixture(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		now = now.Add(2 * time.Minute)
		_, err = service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: artifact.Artifact, Name: "Expired", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		})
		if !errors.Is(err, ErrPairingExpired) || len(store.byID) != 0 {
			t.Fatalf("RedeemPairing(expired) error = %v, devices = %d", err, len(store.byID))
		}
	})

	t.Run("Should mask malformed and unknown artifacts identically (UT-033)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		if _, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal}); err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		for _, artifact := range []string{"malformed", pairingArtifactPrefix + strings.Repeat("A", 43)} {
			_, err := service.RedeemPairing(t.Context(), RedeemRequest{
				Artifact: artifact, Name: "Unknown", Kind: ActorKindOperatorDevice,
				Source: PairingSourcePrivate,
			})
			if !errors.Is(err, ErrPairingInvalid) {
				t.Fatalf("RedeemPairing(%q) error = %v", artifact, err)
			}
		}
	})

	t.Run("Should admit exactly one concurrent redeemer (UT-034)", func(t *testing.T) {
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service, store := newDeviceServiceFixture(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		var wait sync.WaitGroup
		for range 2 {
			wait.Go(func() {
				<-start
				_, redeemErr := service.RedeemPairing(context.Background(), RedeemRequest{
					Artifact: artifact.Artifact, Name: "Race", Kind: ActorKindOperatorDevice,
					Source: PairingSourcePrivate,
				})
				results <- redeemErr
			})
		}
		close(start)
		wait.Wait()
		close(results)
		var succeeded, spent int
		for result := range results {
			if result == nil {
				succeeded++
			} else if errors.Is(result, ErrPairingSpent) {
				spent++
			}
		}
		if succeeded != 1 || spent != 1 || len(store.byID) != 1 {
			t.Fatalf("race results = success:%d spent:%d devices:%d", succeeded, spent, len(store.byID))
		}
	})

	t.Run("Should bound pending artifacts and free capacity after expiry (UT-035)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now, WithPairingLimits(1, time.Minute))
		if _, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal}); err != nil {
			t.Fatalf("MintPairing(first) error = %v", err)
		}
		if _, err := service.MintPairing(
			t.Context(),
			PairingRequest{Source: PairingSourceLocal},
		); !errors.Is(err, ErrPairingLimit) {
			t.Fatalf("MintPairing(over limit) error = %v", err)
		}
		now = now.Add(2 * time.Minute)
		if _, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal}); err != nil {
			t.Fatalf("MintPairing(after expiry) error = %v", err)
		}
	})

	t.Run("Should free pending capacity after a pairing is consumed (UT-035)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now, WithPairingLimits(1, time.Minute))
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing(first) error = %v", err)
		}
		issued, err := service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: artifact.Artifact, Name: "Consumed", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		})
		if err != nil {
			t.Fatalf("RedeemPairing() error = %v", err)
		}
		if issued.Device.PairingOrigin != string(PairingSourceLocal) {
			t.Fatalf("PairingOrigin = %q, want trusted mint source %q", issued.Device.PairingOrigin, PairingSourceLocal)
		}
		if _, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal}); err != nil {
			t.Fatalf("MintPairing(after consume) error = %v", err)
		}
		if got := len(service.pairings.entries); got > 1 {
			t.Fatalf("pairing entries = %d, want at most max_pending", got)
		}
	})

	t.Run("Should allow local mint with no exposure and refuse public pairing (UT-036)", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		if _, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal}); err != nil {
			t.Fatalf("MintPairing(local) error = %v", err)
		}
		if _, err := service.MintPairing(
			t.Context(),
			PairingRequest{Source: PairingSourcePublic},
		); !errors.Is(err, ErrPairingForbidden) {
			t.Fatalf("MintPairing(public) error = %v", err)
		}
	})
}
