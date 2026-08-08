package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	gatewayProfileTransactionJournalVersion = 2
	gatewayProfileTransactionJournalMaxSize = 4 * 1024
	gatewayProfileTransactionStateLockName  = "_gateway_profiles"
	gatewayProfileTransactionBackupPrefix   = "_transaction_backup_"
	gatewayProfileTransactionBackupBytes    = 18
)

type gatewayProfileTransactionOperation string

const (
	gatewayProfileTransactionUpsert gatewayProfileTransactionOperation = "upsert"
	gatewayProfileTransactionRemove gatewayProfileTransactionOperation = "remove"
	gatewayProfileTransactionPair   gatewayProfileTransactionOperation = "pair"
)

// gatewayProfileTransactionJournal intentionally contains only a credential digest.
type gatewayProfileTransactionJournal struct {
	Version                  int                                    `json:"version"`
	Profile                  string                                 `json:"profile"`
	Operation                gatewayProfileTransactionOperation     `json:"operation"`
	DesiredProfile           *compozyconfig.GatewayConnectionConfig `json:"desired_profile,omitempty"`
	DesiredActive            string                                 `json:"desired_active"`
	CredentialSHA256         string                                 `json:"credential_sha256,omitempty"`
	PreviousProfile          *compozyconfig.GatewayConnectionConfig `json:"previous_profile,omitempty"`
	PreviousActive           string                                 `json:"previous_active"`
	PreviousCredentialSHA256 string                                 `json:"previous_credential_sha256,omitempty"`
	PreviousCredentialBackup string                                 `json:"previous_credential_backup,omitempty"`
	PairingStartedAt         time.Time                              `json:"pairing_started_at,omitzero"`
}

type gatewayProfileTransactionPlan struct {
	operation          gatewayProfileTransactionOperation
	profile            compozyconfig.GatewayConnectionConfig
	desiredActive      string
	credential         string
	previousProfile    compozyconfig.GatewayConnectionConfig
	previousActive     string
	previousCredential string
	previousExists     bool
	pairingStartedAt   time.Time
}

type gatewayProfileTransaction struct {
	credentialsDir string
	journal        gatewayProfileTransactionJournal
	lock           *gatewayProfileTransactionLock
}

type gatewayProfileTransactionRecovery func(context.Context, gatewayProfileTransactionJournal) error

func beginGatewayProfileTransactionWithLock(
	credentialsDir string,
	plan gatewayProfileTransactionPlan,
	lock *gatewayProfileTransactionLock,
) (*gatewayProfileTransaction, error) {
	if lock == nil || lock.lock == nil {
		return nil, errors.New("cli: gateway profile transaction lock is required")
	}
	if err := ensureGatewayProfileTransactionReady(credentialsDir, plan.profile.Name); err != nil {
		return nil, err
	}
	journal, err := newGatewayProfileTransactionJournal(plan)
	if err != nil {
		return nil, err
	}
	if err := writeGatewayProfileTransactionJournal(credentialsDir, journal); err != nil {
		return nil, err
	}
	return &gatewayProfileTransaction{
		credentialsDir: credentialsDir,
		journal:        journal,
		lock:           lock,
	}, nil
}

func (t *gatewayProfileTransaction) BackupPreviousCredential(
	deps commandDeps,
	credential string,
) error {
	if t == nil {
		return errors.New("cli: gateway profile transaction is required")
	}
	if t.journal.PreviousCredentialBackup == "" {
		return nil
	}
	if _, err := deps.writeGatewayCredential(
		t.credentialsDir,
		t.journal.PreviousCredentialBackup,
		credential,
	); err != nil {
		return fmt.Errorf("cli: back up previous gateway credential: %w", err)
	}
	return nil
}

func (t *gatewayProfileTransaction) Complete(deps commandDeps) error {
	if t == nil {
		return errors.New("cli: gateway profile transaction is required")
	}
	if err := removeGatewayProfileTransactionBackup(t.credentialsDir, t.journal, deps); err != nil {
		return err
	}
	if err := removeGatewayProfileTransactionJournal(t.credentialsDir, t.journal.Profile); err != nil {
		return err
	}
	return t.Release()
}

func (t *gatewayProfileTransaction) Release() error {
	if t == nil || t.lock == nil {
		return nil
	}
	if err := t.lock.Release(); err != nil {
		return err
	}
	t.lock = nil
	return nil
}

func newGatewayProfileTransactionJournal(plan gatewayProfileTransactionPlan) (gatewayProfileTransactionJournal, error) {
	journal := gatewayProfileTransactionJournal{
		Version:          gatewayProfileTransactionJournalVersion,
		Profile:          strings.TrimSpace(plan.profile.Name),
		Operation:        plan.operation,
		DesiredActive:    strings.TrimSpace(plan.desiredActive),
		PreviousActive:   strings.TrimSpace(plan.previousActive),
		PairingStartedAt: plan.pairingStartedAt.UTC(),
	}
	if plan.operation == gatewayProfileTransactionUpsert || plan.operation == gatewayProfileTransactionPair {
		profile := plan.profile
		journal.DesiredProfile = &profile
	}
	if plan.previousExists {
		profile := plan.previousProfile
		journal.PreviousProfile = &profile
	}
	credential := strings.TrimSpace(plan.credential)
	if credential != "" {
		journal.CredentialSHA256 = gatewayProfileCredentialDigest(credential)
	}
	previousCredential := strings.TrimSpace(plan.previousCredential)
	if previousCredential != "" {
		journal.PreviousCredentialSHA256 = gatewayProfileCredentialDigest(previousCredential)
		backupName, err := newGatewayProfileTransactionBackupName()
		if err != nil {
			return gatewayProfileTransactionJournal{}, err
		}
		journal.PreviousCredentialBackup = backupName
	}
	if err := validateGatewayProfileTransactionJournal(journal); err != nil {
		return gatewayProfileTransactionJournal{}, err
	}
	return journal, nil
}

func gatewayProfileCredentialDigest(credential string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(credential)))
	return hex.EncodeToString(sum[:])
}

func recoverGatewayProfileTransactions(
	ctx context.Context,
	credentialsDir string,
	recoverJournal gatewayProfileTransactionRecovery,
) error {
	if ctx == nil {
		return errors.New("cli: gateway profile transaction recovery context is required")
	}
	if recoverJournal == nil {
		return errors.New("cli: gateway profile transaction recovery callback is required")
	}
	lock, err := acquireGatewayProfileTransactionLock(ctx, credentialsDir, gatewayProfileTransactionStateLockName)
	if err != nil {
		return err
	}
	operationErr := recoverGatewayProfileTransactionsWithLock(ctx, credentialsDir, recoverJournal, lock)
	releaseErr := lock.Release()
	return errors.Join(operationErr, releaseErr)
}

func recoverGatewayProfileTransactionsWithLock(
	ctx context.Context,
	credentialsDir string,
	recoverJournal gatewayProfileTransactionRecovery,
	lock *gatewayProfileTransactionLock,
) error {
	return recoverGatewayProfileTransactionsWithPolicy(
		ctx,
		credentialsDir,
		recoverJournal,
		lock,
		nil,
	)
}

func recoverAvailableGatewayProfileTransactionsWithLock(
	ctx context.Context,
	credentialsDir string,
	recoverJournal gatewayProfileTransactionRecovery,
	lock *gatewayProfileTransactionLock,
) error {
	return recoverGatewayProfileTransactionsWithPolicy(
		ctx,
		credentialsDir,
		recoverJournal,
		lock,
		isGatewayPairingRecoveryPending,
	)
}

func recoverGatewayProfileTransactionsWithPolicy(
	ctx context.Context,
	credentialsDir string,
	recoverJournal gatewayProfileTransactionRecovery,
	lock *gatewayProfileTransactionLock,
	shouldContinue func(error) bool,
) error {
	if lock == nil || lock.lock == nil {
		return errors.New("cli: gateway profile transaction lock is required")
	}
	journals, err := listGatewayProfileTransactionJournals(credentialsDir)
	if err != nil {
		return err
	}
	for _, listed := range journals {
		if err := recoverGatewayProfileTransactionJournal(
			ctx,
			credentialsDir,
			listed.Profile,
			recoverJournal,
		); err != nil {
			if shouldContinue != nil && shouldContinue(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func ensureGatewayProfileTransactionReady(credentialsDir, profile string) error {
	journal, err := readGatewayProfileTransactionJournal(credentialsDir, strings.TrimSpace(profile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if journal.Operation == gatewayProfileTransactionPair {
		return newGatewayPairingRecoveryPendingError(nil)
	}
	return fmt.Errorf("cli: gateway profile transaction %q is still pending", journal.Profile)
}

func recoverGatewayProfileTransactionJournal(
	ctx context.Context,
	credentialsDir string,
	profile string,
	recoverJournal gatewayProfileTransactionRecovery,
) error {
	journal, err := readGatewayProfileTransactionJournal(credentialsDir, profile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := recoverJournal(ctx, journal); err != nil {
		return fmt.Errorf("cli: recover gateway profile transaction %q: %w", profile, err)
	}
	return removeGatewayProfileTransactionJournal(credentialsDir, profile)
}

func decodeGatewayProfileTransactionJournal(contents []byte) (gatewayProfileTransactionJournal, error) {
	if len(contents) == 0 || len(contents) > gatewayProfileTransactionJournalMaxSize {
		return gatewayProfileTransactionJournal{}, errors.New(
			"cli: gateway profile transaction journal size is invalid",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var journal gatewayProfileTransactionJournal
	if err := decoder.Decode(&journal); err != nil {
		return gatewayProfileTransactionJournal{}, fmt.Errorf(
			"cli: decode gateway profile transaction journal: %w",
			err,
		)
	}
	if err := ensureGatewayProfileTransactionJournalEOF(decoder); err != nil {
		return gatewayProfileTransactionJournal{}, err
	}
	if err := validateGatewayProfileTransactionJournal(journal); err != nil {
		return gatewayProfileTransactionJournal{}, err
	}
	return journal, nil
}

func ensureGatewayProfileTransactionJournalEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("cli: gateway profile transaction journal contains multiple values")
		}
		return fmt.Errorf("cli: decode trailing gateway profile transaction journal data: %w", err)
	}
	return nil
}

func validateGatewayProfileTransactionJournal(journal gatewayProfileTransactionJournal) error {
	if err := validateGatewayProfileTransactionJournalIdentity(journal); err != nil {
		return err
	}
	if err := validateGatewayProfileTransactionJournalActiveProfiles(journal); err != nil {
		return err
	}
	if err := validateGatewayProfileTransactionJournalDesiredState(journal); err != nil {
		return err
	}
	if err := validateGatewayProfileTransactionJournalPreviousState(journal); err != nil {
		return err
	}
	return validateGatewayProfileTransactionJournalCredentialDigests(journal)
}

func validateGatewayProfileTransactionJournalIdentity(journal gatewayProfileTransactionJournal) error {
	if journal.Version != gatewayProfileTransactionJournalVersion {
		return errors.New("cli: gateway profile transaction journal version is invalid")
	}
	if !validGatewayProfileName(journal.Profile) {
		return errors.New("cli: gateway profile transaction journal profile is invalid")
	}
	if journal.Operation != gatewayProfileTransactionUpsert &&
		journal.Operation != gatewayProfileTransactionRemove &&
		journal.Operation != gatewayProfileTransactionPair {
		return errors.New("cli: gateway profile transaction journal operation is invalid")
	}
	return nil
}

func validateGatewayProfileTransactionJournalActiveProfiles(journal gatewayProfileTransactionJournal) error {
	if strings.TrimSpace(journal.DesiredActive) != "" && !validGatewayProfileName(journal.DesiredActive) {
		return errors.New("cli: gateway profile transaction desired active profile is invalid")
	}
	if strings.TrimSpace(journal.PreviousActive) != "" && !validGatewayProfileName(journal.PreviousActive) {
		return errors.New("cli: gateway profile transaction previous active profile is invalid")
	}
	return nil
}

func validateGatewayProfileTransactionJournalDesiredState(journal gatewayProfileTransactionJournal) error {
	switch journal.Operation {
	case gatewayProfileTransactionUpsert, gatewayProfileTransactionPair:
		if journal.DesiredProfile == nil || journal.DesiredProfile.Name != journal.Profile {
			return errors.New("cli: gateway profile upsert journal desired profile is invalid")
		}
		if err := journal.DesiredProfile.Validate(); err != nil {
			return fmt.Errorf("cli: gateway profile upsert journal desired profile: %w", err)
		}
		if journal.CredentialSHA256 == "" {
			return errors.New("cli: gateway profile upsert journal credential digest is required")
		}
		if journal.Operation == gatewayProfileTransactionPair {
			if journal.DesiredProfile.Scheme != gatewaySchemeHTTPS || journal.PairingStartedAt.IsZero() {
				return errors.New("cli: gateway pairing journal state is invalid")
			}
		} else if !journal.PairingStartedAt.IsZero() {
			return errors.New("cli: gateway profile upsert journal contains pairing state")
		}
		return nil
	case gatewayProfileTransactionRemove:
		if journal.DesiredProfile != nil || journal.CredentialSHA256 != "" || !journal.PairingStartedAt.IsZero() {
			return errors.New("cli: gateway profile removal journal contains upsert state")
		}
		return nil
	default:
		return errors.New("cli: gateway profile transaction journal operation is invalid")
	}
}

func validateGatewayProfileTransactionJournalPreviousState(journal gatewayProfileTransactionJournal) error {
	switch {
	case journal.PreviousProfile != nil:
		if journal.PreviousProfile.Name != journal.Profile {
			return errors.New("cli: gateway profile transaction previous profile is invalid")
		}
		if err := journal.PreviousProfile.Validate(); err != nil {
			return fmt.Errorf("cli: gateway profile transaction previous profile: %w", err)
		}
		if journal.PreviousProfile.Scheme == gatewaySchemeHTTPS && journal.PreviousCredentialSHA256 == "" {
			return errors.New("cli: gateway profile transaction previous credential digest is required")
		}
		if journal.PreviousProfile.Scheme == gatewaySchemeHTTPS &&
			!validGatewayProfileName(journal.PreviousCredentialBackup) {
			return errors.New("cli: gateway profile transaction previous credential backup is invalid")
		}
	case journal.PreviousCredentialSHA256 != "":
		return errors.New("cli: gateway profile transaction previous credential has no profile")
	case journal.PreviousCredentialBackup != "":
		return errors.New("cli: gateway profile transaction previous credential backup has no profile")
	}
	return nil
}

func newGatewayProfileTransactionBackupName() (string, error) {
	raw := make([]byte, gatewayProfileTransactionBackupBytes)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", fmt.Errorf("cli: generate gateway credential backup name: %w", err)
	}
	return gatewayProfileTransactionBackupPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func removeGatewayProfileTransactionBackup(
	credentialsDir string,
	journal gatewayProfileTransactionJournal,
	deps commandDeps,
) error {
	if journal.PreviousCredentialBackup == "" {
		return nil
	}
	if deps.removeGatewayCredential == nil {
		return errors.New("cli: gateway credential removal dependency is required")
	}
	if err := deps.removeGatewayCredential(credentialsDir, journal.PreviousCredentialBackup); err != nil {
		return fmt.Errorf("cli: remove previous gateway credential backup: %w", err)
	}
	return nil
}

func validateGatewayProfileTransactionJournalCredentialDigests(journal gatewayProfileTransactionJournal) error {
	if journal.CredentialSHA256 != "" && !validGatewayProfileCredentialDigest(journal.CredentialSHA256) {
		return errors.New("cli: gateway profile transaction journal credential digest is invalid")
	}
	if journal.PreviousCredentialSHA256 != "" &&
		!validGatewayProfileCredentialDigest(journal.PreviousCredentialSHA256) {
		return errors.New("cli: gateway profile transaction journal previous credential digest is invalid")
	}
	return nil
}

func validGatewayProfileCredentialDigest(digest string) bool {
	if len(digest) != sha256.Size*2 || strings.ToLower(digest) != digest {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}
