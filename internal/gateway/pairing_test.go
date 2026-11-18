package gateway

import (
	"bytes"
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
				_, redeemErr := service.RedeemPairing(t.Context(), RedeemRequest{
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

	// Invariant: the manual code is a uniformly random 6-digit alias of one pending artifact.
	// Owning layer: gateway pairing store mint.
	// Canonical suite: this device credential and pairing contract suite.
	t.Run("Should mint a 6-digit manual code beside the opaque artifact", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		if !validPairingManualCode(artifact.ManualCode) {
			t.Fatalf("manual code = %q, want exactly 6 digits", artifact.ManualCode)
		}
		if artifact.ManualCode == artifact.Artifact {
			t.Fatal("manual code reused the opaque artifact")
		}
	})

	t.Run("Should keep manual code draws uniform by rejecting the biased tail", func(t *testing.T) {
		t.Parallel()
		reader := bytes.NewReader([]byte{
			0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff,
			0x00, 0x0f, 0x42, 0x3f,
		})
		code, err := generatePairingManualCode(reader)
		if err != nil || code != "999999" {
			t.Fatalf("generatePairingManualCode() = %q, %v, want rejected draws skipped", code, err)
		}
		zero, err := generatePairingManualCode(bytes.NewReader([]byte{0, 0, 0, 0}))
		if err != nil || zero != "000000" {
			t.Fatalf("generatePairingManualCode(zero) = %q, %v, want zero-padded code", zero, err)
		}
		seen := make(map[string]struct{}, 200)
		for range 200 {
			code, err := generatePairingManualCode(defaultEntropy())
			if err != nil || !validPairingManualCode(code) {
				t.Fatalf("generatePairingManualCode(random) = %q, %v", code, err)
			}
			seen[code] = struct{}{}
		}
		if len(seen) < 100 {
			t.Fatalf("distinct codes = %d of 200, want a non-degenerate distribution", len(seen))
		}
	})

	t.Run("Should retry a colliding manual code instead of reusing it", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		stream := append(bytes.Repeat([]byte{0x02}, secretEntropyBytes), []byte{
			0x00, 0x01, 0xe2, 0x40, // 123456 collides with the seeded code
			0x00, 0x09, 0xfb, 0xf1, // 654321 is the retry
		}...)
		store, err := newPairingStore(4, time.Minute, func() time.Time { return now }, bytes.NewReader(stream))
		if err != nil {
			t.Fatalf("newPairingStore() error = %v", err)
		}
		store.manual[hashCredential("123456")] = "seeded-digest"
		artifact, err := store.Mint(PairingSourceLocal)
		if err != nil {
			t.Fatalf("Mint() error = %v", err)
		}
		if artifact.ManualCode != "654321" {
			t.Fatalf("manual code = %q, want the collision-free retry", artifact.ManualCode)
		}
		if got := store.manual[hashCredential("654321")]; got != hashCredential(artifact.Artifact) {
			t.Fatalf("manual index = %q, want the artifact digest", got)
		}
	})

	t.Run("Should fail closed when every manual code draw collides", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		stream := bytes.Repeat([]byte{0x02}, secretEntropyBytes)
		for range pairingManualCodeAttempts {
			stream = append(stream, 0x00, 0x01, 0xe2, 0x40)
		}
		store, err := newPairingStore(4, time.Minute, func() time.Time { return now }, bytes.NewReader(stream))
		if err != nil {
			t.Fatalf("newPairingStore() error = %v", err)
		}
		store.manual[hashCredential("123456")] = "seeded-digest"
		if _, err := store.Mint(PairingSourceLocal); err == nil ||
			!strings.Contains(err.Error(), "collided") {
			t.Fatalf("Mint() error = %v, want a wrapped collision failure", err)
		}
		if len(store.entries) != 0 || len(store.manual) != 1 {
			t.Fatalf(
				"pairing state = %d entries / %d codes, want no partial mint",
				len(store.entries),
				len(store.manual),
			)
		}
	})

	// Invariant: either representation redeems the same pending entry exactly once.
	// Owning layer: gateway pairing store redeem.
	// Canonical suite: this device credential and pairing contract suite.
	t.Run("Should redeem by manual code and spend the artifact too", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		issued, err := service.RedeemPairing(t.Context(), RedeemRequest{
			ManualCode: artifact.ManualCode, Name: "Typed", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		})
		if err != nil {
			t.Fatalf("RedeemPairing(manual code) error = %v", err)
		}
		if issued.Device.Name != "Typed" {
			t.Fatalf("issued device = %#v", issued.Device)
		}
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: artifact.Artifact, Name: "Replay", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); !errors.Is(err, ErrPairingSpent) {
			t.Fatalf("RedeemPairing(artifact after code) error = %v, want ErrPairingSpent", err)
		}
	})

	t.Run("Should spend the manual code when the artifact redeems first", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			Artifact: artifact.Artifact, Name: "Artifact", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); err != nil {
			t.Fatalf("RedeemPairing(artifact) error = %v", err)
		}
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			ManualCode: artifact.ManualCode, Name: "Replay", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); !errors.Is(err, ErrPairingSpent) {
			t.Fatalf("RedeemPairing(code after artifact) error = %v, want ErrPairingSpent", err)
		}
	})

	t.Run("Should normalize a grouped manual code before redemption", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		grouped := artifact.ManualCode[:3] + " " + artifact.ManualCode[3:]
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			ManualCode: grouped, Name: "Grouped", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); err != nil {
			t.Fatalf("RedeemPairing(grouped code) error = %v", err)
		}
	})

	t.Run("Should report expiry and mask unknown manual codes", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		service := newDeviceServiceOnly(t, &now)
		artifact, err := service.MintPairing(t.Context(), PairingRequest{Source: PairingSourceLocal})
		if err != nil {
			t.Fatalf("MintPairing() error = %v", err)
		}
		unknown := artifact.ManualCode[:5] + string((artifact.ManualCode[5]-'0'+1)%10+'0')
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			ManualCode: unknown, Name: "Unknown", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); !errors.Is(err, ErrPairingInvalid) {
			t.Fatalf("RedeemPairing(unknown code) error = %v, want ErrPairingInvalid", err)
		}
		now = now.Add(6 * time.Minute)
		if _, err := service.RedeemPairing(t.Context(), RedeemRequest{
			ManualCode: artifact.ManualCode, Name: "Expired", Kind: ActorKindOperatorDevice,
			Source: PairingSourcePrivate,
		}); !errors.Is(err, ErrPairingExpired) {
			t.Fatalf("RedeemPairing(expired code) error = %v, want ErrPairingExpired", err)
		}
		if len(service.pairings.manual) != 0 {
			t.Fatalf(
				"manual index = %d, want expired entries evicted with their artifact",
				len(service.pairings.manual),
			)
		}
	})
}
