package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Service resolves env-backed and vault-backed secret references.
type Service struct {
	store     Store
	keys      KeyProvider
	lookupEnv func(string) (string, bool)
	now       func() time.Time
}

// Option customizes the vault service.
type Option func(*Service)

// WithLookupEnv injects env lookup for tests and daemon composition.
func WithLookupEnv(lookup func(string) (string, bool)) Option {
	return func(service *Service) {
		service.lookupEnv = lookup
	}
}

// WithNow injects the service clock.
func WithNow(now func() time.Time) Option {
	return func(service *Service) {
		service.now = now
	}
}

// NewService constructs a vault service.
func NewService(store Store, keys KeyProvider, opts ...Option) (*Service, error) {
	if store == nil {
		return nil, errors.New("vault: store is required")
	}
	if keys == nil {
		return nil, errors.New("vault: key provider is required")
	}
	service := &Service{
		store:     store,
		keys:      keys,
		lookupEnv: os.LookupEnv,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	if service.lookupEnv == nil {
		service.lookupEnv = os.LookupEnv
	}
	if service.now == nil {
		service.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return service, nil
}

// PutSecret encrypts and stores one vault-backed secret ref.
func (s *Service) PutSecret(ctx context.Context, ref string, kind string, plaintext string) (Metadata, error) {
	if ctx == nil {
		return Metadata{}, errors.New("vault: put secret context is required")
	}
	normalized := NormalizeRef(ref)
	if err := ValidateSecretRef(normalized); err != nil {
		return Metadata{}, err
	}
	normalizedKind := strings.TrimSpace(kind)
	if normalizedKind == "" {
		existing, err := s.store.GetVaultSecret(ctx, normalized)
		switch {
		case err == nil:
			normalizedKind = existing.Kind
		case errors.Is(err, ErrSecretNotFound):
		default:
			return Metadata{}, err
		}
	}
	key, err := s.keys.Key()
	if err != nil {
		return Metadata{}, err
	}
	encrypted, err := encryptValue(key, plaintext)
	if err != nil {
		return Metadata{}, err
	}
	now := s.now()
	record := Record{
		Ref:            normalized,
		Kind:           normalizedKind,
		EncryptedValue: encrypted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.PutVaultSecret(ctx, record); err != nil {
		return Metadata{}, err
	}
	stored, err := s.store.GetVaultSecret(ctx, normalized)
	if err != nil {
		return Metadata{}, err
	}
	return metadataForRecord(stored), nil
}

// ResolveRef resolves env: and vault: refs to plaintext for launch-time injection.
func (s *Service) ResolveRef(ctx context.Context, ref string) (string, error) {
	if ctx == nil {
		return "", errors.New("vault: resolve ref context is required")
	}
	normalized := NormalizeRef(ref)
	switch {
	case IsEnvRef(normalized):
		envName, err := EnvNameFromRef(normalized)
		if err != nil {
			return "", err
		}
		value, ok := s.lookupEnv(envName)
		if !ok || strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("%w: env:%s", ErrMissingSecret, envName)
		}
		return value, nil
	case IsSecretRef(normalized):
		if err := ValidateSecretRef(normalized); err != nil {
			return "", err
		}
		record, err := s.store.GetVaultSecret(ctx, normalized)
		if err != nil {
			return "", err
		}
		key, err := s.keys.Key()
		if err != nil {
			return "", err
		}
		value, err := decryptValue(key, record.EncryptedValue)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("%w: %s", ErrMissingSecret, normalized)
		}
		return value, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedSecretRef, normalized)
	}
}

// GetMetadata returns redacted metadata for one vault-backed ref.
func (s *Service) GetMetadata(ctx context.Context, ref string) (Metadata, error) {
	normalized := NormalizeRef(ref)
	if err := ValidateSecretRef(normalized); err != nil {
		return Metadata{}, err
	}
	record, err := s.store.GetVaultSecret(ctx, normalized)
	if err != nil {
		return Metadata{}, err
	}
	return metadataForRecord(record), nil
}

// ListMetadata returns redacted metadata for a ref prefix.
func (s *Service) ListMetadata(ctx context.Context, prefix string) ([]Metadata, error) {
	normalizedPrefix := NormalizeRef(prefix)
	if err := ValidateSecretRefPrefix(normalizedPrefix); err != nil {
		return nil, err
	}
	records, err := s.store.ListVaultSecrets(ctx, normalizedPrefix)
	if err != nil {
		return nil, err
	}
	values := make([]Metadata, 0, len(records))
	for _, record := range records {
		values = append(values, metadataForRecord(record))
	}
	return values, nil
}

// DeleteSecret removes one vault-backed ref.
func (s *Service) DeleteSecret(ctx context.Context, ref string) error {
	normalized := NormalizeRef(ref)
	if err := ValidateSecretRef(normalized); err != nil {
		return err
	}
	return s.store.DeleteVaultSecret(ctx, normalized)
}

func metadataForRecord(record Record) Metadata {
	return Metadata{
		Ref:       NormalizeRef(record.Ref),
		Kind:      strings.TrimSpace(record.Kind),
		Present:   strings.TrimSpace(record.EncryptedValue) != "",
		CreatedAt: record.CreatedAt,
		UpdatedAt: record.UpdatedAt,
	}
}
