package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/fileutil"
)

// Invariant: one profile transaction holds the cross-process lock until it releases it.
// Owning layer: integration over the filesystem and advisory file-lock boundary.
// Canonical suite: this new gateway profile transaction boundary suite.
func TestGatewayProfileTransactionLock(t *testing.T) {
	t.Parallel()

	t.Run("Should serialize concurrent access to one profile", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		first, err := acquireGatewayProfileTransactionLock(t.Context(), directory, "laptop")
		if err != nil {
			t.Fatalf("acquire first lock error = %v", err)
		}
		t.Cleanup(func() {
			if releaseErr := first.Release(); releaseErr != nil {
				t.Errorf("release first lock error = %v", releaseErr)
			}
		})

		type lockResult struct {
			lock *gatewayProfileTransactionLock
			err  error
		}
		started := make(chan struct{})
		result := make(chan lockResult, 1)
		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		go func() {
			close(started)
			lock, acquireErr := acquireGatewayProfileTransactionLock(ctx, directory, "laptop")
			result <- lockResult{lock: lock, err: acquireErr}
		}()
		<-started

		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()
		select {
		case second := <-result:
			if second.lock != nil {
				releaseErr := second.lock.Release()
				if releaseErr != nil {
					t.Errorf("release prematurely acquired lock error = %v", releaseErr)
				}
			}
			t.Fatalf("second lock completed before first release: %v", second.err)
		case <-timer.C:
		}

		if err := first.Release(); err != nil {
			t.Fatalf("release first lock error = %v", err)
		}
		second := <-result
		if second.err != nil {
			t.Fatalf("acquire second lock error = %v", second.err)
		}
		if second.lock == nil {
			t.Fatal("acquire second lock returned nil lock")
		}
		if err := second.lock.Release(); err != nil {
			t.Fatalf("release second lock error = %v", err)
		}
	})
}

// Invariant: journals expose only the credential digest and are private, atomic recovery records.
// Owning layer: integration over the credential-directory filesystem boundary.
// Canonical suite: this new gateway profile transaction boundary suite.
func TestGatewayProfileTransactionJournal(t *testing.T) {
	t.Parallel()

	t.Run("Should persist an encrypted-credential digest with strict permissions", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		credential := "cpz_gwd_" + string(bytes.Repeat([]byte{'x'}, 43))
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionUpsert,
			profile: compozyconfig.GatewayConnectionConfig{
				Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
				CredentialFile: "laptop.cred",
			},
			credential: credential,
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal() error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(directory, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal() error = %v", err)
		}

		path, err := gatewayProfileTransactionJournalPath(directory, "laptop")
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(journal) error = %v", err)
		}
		if bytes.Contains(contents, []byte(credential)) {
			t.Fatal("journal contains the raw credential")
		}
		sum := sha256.Sum256([]byte(credential))
		if !bytes.Contains(contents, []byte(hex.EncodeToString(sum[:]))) {
			t.Fatal("journal does not contain the credential digest")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(journal) error = %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("journal permissions = %o, want 600", got)
		}

		got, err := readGatewayProfileTransactionJournal(directory, "laptop")
		if err != nil {
			t.Fatalf("readGatewayProfileTransactionJournal() error = %v", err)
		}
		if !reflect.DeepEqual(got, journal) {
			t.Fatalf("journal = %#v, want %#v", got, journal)
		}
		journals, err := listGatewayProfileTransactionJournals(directory)
		if err != nil {
			t.Fatalf("listGatewayProfileTransactionJournals() error = %v", err)
		}
		if len(journals) != 1 || !reflect.DeepEqual(journals[0], journal) {
			t.Fatalf("journals = %#v, want [%#v]", journals, journal)
		}
	})

	t.Run("Should reject symbolic links during journal read and enumeration", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionRemove,
			profile:   compozyconfig.GatewayConnectionConfig{Name: "laptop"},
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal() error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(directory, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal() error = %v", err)
		}
		target, err := gatewayProfileTransactionJournalPath(directory, "laptop")
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		linkedPath, err := gatewayProfileTransactionJournalPath(directory, "linked")
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		if err := os.Symlink(target, linkedPath); err != nil {
			t.Fatalf("Symlink() error = %v", err)
		}

		if _, err := readGatewayProfileTransactionJournal(directory, "linked"); !errors.Is(err, fileutil.ErrSymlink) {
			t.Fatalf("readGatewayProfileTransactionJournal() error = %v, want fileutil.ErrSymlink", err)
		}
		if _, err := listGatewayProfileTransactionJournals(directory); !errors.Is(err, fileutil.ErrSymlink) {
			t.Fatalf("listGatewayProfileTransactionJournals() error = %v, want fileutil.ErrSymlink", err)
		}
	})

	t.Run("Should reject corrupt journals before recovery", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		if err := ensureGatewayProfileTransactionDirectory(directory); err != nil {
			t.Fatalf("ensureGatewayProfileTransactionDirectory() error = %v", err)
		}
		path, err := gatewayProfileTransactionJournalPath(directory, "laptop")
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(`{"version":`), 0o600); err != nil {
			t.Fatalf("WriteFile(corrupt journal) error = %v", err)
		}

		if _, err := readGatewayProfileTransactionJournal(directory, "laptop"); err == nil {
			t.Fatal("readGatewayProfileTransactionJournal() error = nil, want corruption rejection")
		}
		if _, err := listGatewayProfileTransactionJournals(directory); err == nil {
			t.Fatal("listGatewayProfileTransactionJournals() error = nil, want corruption rejection")
		}
	})
}

// Invariant: recovery retains a durable journal until its resolver completes successfully.
// Owning layer: integration over the credential-directory filesystem boundary.
// Canonical suite: this new gateway profile transaction boundary suite.
func TestGatewayProfileTransactionRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should remove a journal only after its resolver succeeds", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionRemove,
			profile:   compozyconfig.GatewayConnectionConfig{Name: "laptop"},
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal() error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(directory, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal() error = %v", err)
		}
		resolverErr := errors.New("recovery failed")
		if err := recoverGatewayProfileTransactions(t.Context(), directory, func(
			context.Context,
			gatewayProfileTransactionJournal,
		) error {
			return resolverErr
		}); !errors.Is(err, resolverErr) {
			t.Fatalf("recoverGatewayProfileTransactions() error = %v, want resolver error", err)
		}
		if _, err := readGatewayProfileTransactionJournal(directory, "laptop"); err != nil {
			t.Fatalf("journal disappeared after failed recovery: %v", err)
		}

		var recovered gatewayProfileTransactionJournal
		if err := recoverGatewayProfileTransactions(t.Context(), directory, func(
			ctx context.Context,
			got gatewayProfileTransactionJournal,
		) error {
			if ctx == nil {
				t.Fatal("recovery callback received nil context")
			}
			recovered = got
			return nil
		}); err != nil {
			t.Fatalf("recoverGatewayProfileTransactions() error = %v", err)
		}
		if !reflect.DeepEqual(recovered, journal) {
			t.Fatalf("recovered journal = %#v, want %#v", recovered, journal)
		}
		path, err := gatewayProfileTransactionJournalPath(directory, "laptop")
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(recovered journal) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should retain an uncertain pairing until authentication or expiry resolves it", func(t *testing.T) {
		t.Parallel()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		startedAt := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
		credential := testGatewayCredential('u')
		profile := compozyconfig.GatewayConnectionConfig{
			Name: "laptop", Scheme: "https", Host: "gateway.example", Port: 443,
			CredentialFile: "laptop.cred",
		}
		journal, err := newGatewayProfileTransactionJournal(gatewayProfileTransactionPlan{
			operation: gatewayProfileTransactionPair, profile: profile,
			credential: credential, pairingStartedAt: startedAt,
		})
		if err != nil {
			t.Fatalf("newGatewayProfileTransactionJournal(pair) error = %v", err)
		}
		if err := writeGatewayProfileTransactionJournal(homePaths.GatewayCredentialsDir, journal); err != nil {
			t.Fatalf("writeGatewayProfileTransactionJournal(pair) error = %v", err)
		}
		credentials := map[string]string{profile.Name: credential}
		now := startedAt.Add(time.Minute)
		gatewayAPI := &gatewayPairingAPIStub{listDevicesErr: &daemonAPIError{
			statusCode: 401,
			payload: contract.ErrorPayload{
				Error: "unauthorized", Code: "gateway_device_unauthenticated",
			},
		}}
		deps := commandDeps{
			now: func() time.Time { return now },
			newClient: func(ClientTarget) (DaemonClient, error) {
				return &gatewayPairingDaemonClient{DaemonClient: &stubClient{}, gatewayClientAPI: gatewayAPI}, nil
			},
			readGatewayCredential: func(_ string, name string) (string, error) {
				value, exists := credentials[name]
				if !exists {
					return "", os.ErrNotExist
				}
				return value, nil
			},
			writeGatewayCredential: func(_ string, name string, value string) (string, error) {
				credentials[name] = value
				return name + ".cred", nil
			},
			removeGatewayCredential: func(_ string, name string) error {
				delete(credentials, name)
				return nil
			},
		}
		err = recoverGatewayProfileState(t.Context(), homePaths, deps)
		assertGatewayClientErrorCode(t, err, "gateway_pairing_recovery_pending")
		if credentials[profile.Name] != credential {
			t.Fatal("uncertain pairing credential was removed before the recovery window")
		}
		if _, err := readGatewayProfileTransactionJournal(
			homePaths.GatewayCredentialsDir,
			profile.Name,
		); err != nil {
			t.Fatalf("pending pairing journal disappeared: %v", err)
		}

		now = startedAt.Add(gatewayPairingRecoveryWindow + time.Second)
		if err := recoverGatewayProfileState(t.Context(), homePaths, deps); err != nil {
			t.Fatalf("recoverGatewayProfileState(expired pairing) error = %v", err)
		}
		if _, exists := credentials[profile.Name]; exists {
			t.Fatal("unredeemed pairing credential still exists after the recovery window")
		}
		path, err := gatewayProfileTransactionJournalPath(homePaths.GatewayCredentialsDir, profile.Name)
		if err != nil {
			t.Fatalf("gatewayProfileTransactionJournalPath() error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(recovered pairing journal) error = %v, want os.ErrNotExist", err)
		}
	})
}

// Invariant: journal paths cannot escape the selected credential directory.
// Owning layer: unit validation at the transaction filesystem boundary.
// Canonical suite: this new gateway profile transaction boundary suite.
func TestGatewayProfileTransactionJournalPath(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unsafe profile names", func(t *testing.T) {
		t.Parallel()

		if _, err := gatewayProfileTransactionJournalPath(
			filepath.Join(t.TempDir(), "credentials"),
			"../escape",
		); err == nil {
			t.Fatal("gatewayProfileTransactionJournalPath() error = nil, want unsafe profile rejection")
		}
	})
}
