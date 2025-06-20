package apikey

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/auth/audit"
	"github.com/compozy/compozy/engine/auth/org"
	"github.com/compozy/compozy/engine/auth/user"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/crypto/argon2"
)

// ServiceConfig holds configuration for the API key service
type ServiceConfig struct {
	// Argon2 parameters
	Time    uint32
	Memory  uint32
	Threads uint8
	KeyLen  uint32
	// Key generation
	KeyLength int    // Number of random bytes to generate (16 bytes = 32 hex chars)
	KeyPrefix string // Prefix for all API keys
}

// DefaultServiceConfig returns secure default configuration
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		Time:      1,
		Memory:    64 * 1024,
		Threads:   4,
		KeyLen:    32,
		KeyLength: KeyLength, // Use constant from domain.go (16 bytes = 32 hex chars)
		KeyPrefix: KeyPrefix, // Use constant from domain.go
	}
}

// Service provides API key generation, hashing, and validation
type Service struct {
	config     ServiceConfig
	apiKeyRepo Repository
	userRepo   user.Repository
	orgRepo    org.Repository
	auditSvc   *audit.Service
}

// NewService creates a new API key service
func NewService(
	config ServiceConfig,
	apiKeyRepo Repository,
	userRepo user.Repository,
	orgRepo org.Repository,
	auditSvc *audit.Service,
) *Service {
	return &Service{
		config:     config,
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
		orgRepo:    orgRepo,
		auditSvc:   auditSvc,
	}
}

// GenerateAPIKey generates a new secure API key with the configured prefix
func (s *Service) GenerateAPIKey() (string, error) {
	keyBytes := make([]byte, s.config.KeyLength)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	key := s.config.KeyPrefix + hex.EncodeToString(keyBytes)
	return key, nil
}

// HashAPIKey hashes an API key using Argon2 with salt
func (s *Service) HashAPIKey(key string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	hash := argon2.IDKey([]byte(key), salt, s.config.Time, s.config.Memory, s.config.Threads, s.config.KeyLen)
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hash), nil
}

// VerifyAPIKey verifies an API key against its hash using constant-time comparison
func (s *Service) VerifyAPIKey(key, hash string) bool {
	parts := strings.Split(hash, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	storedHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	computedHash := argon2.IDKey([]byte(key), salt, s.config.Time, s.config.Memory, s.config.Threads, s.config.KeyLen)
	return subtle.ConstantTimeCompare(storedHash, computedHash) == 1
}

// ExtractKeyPrefix extracts the prefix from an API key for efficient database lookups
func (s *Service) ExtractKeyPrefix(key string) string {
	// Extract first KeyPrefixLookupLength characters after the prefix for database indexing
	if !strings.HasPrefix(key, s.config.KeyPrefix) {
		return ""
	}
	keyWithoutPrefix := strings.TrimPrefix(key, s.config.KeyPrefix)
	if len(keyWithoutPrefix) < KeyPrefixLookupLength {
		return keyWithoutPrefix
	}
	return keyWithoutPrefix[:KeyPrefixLookupLength]
}

// ValidateKey validates an API key and retrieves associated context
func (s *Service) ValidateKey(ctx context.Context, key string) (*APIKey, *user.User, *org.Organization, error) {
	log := logger.FromContext(ctx)
	reqInfo := GetRequestInfo(ctx)
	// Extract prefix for efficient lookup
	prefix := s.ExtractKeyPrefix(key)
	if prefix == "" {
		log.Debug("Invalid API key format - missing prefix")
		s.auditSvc.LogAPIKeyValidationFailed(ctx, "invalid_format", reqInfo.IPAddress, reqInfo.UserAgent, nil)
		return nil, nil, nil, ErrInvalidAPIKey
	}
	// Find API key by prefix across all organizations
	apiKey, err := s.apiKeyRepo.FindByExactPrefix(ctx, prefix)
	if err != nil {
		// Always perform a dummy hash verification to prevent timing attacks
		dummyHash := "dummy:0000000000000000:0000000000000000000000000000000000000000000000000000000000000000"
		s.VerifyAPIKey(key, dummyHash)
		if err == ErrAPIKeyNotFound {
			log.With("prefix", prefix).Debug("API key not found")
			s.auditSvc.LogAPIKeyValidationFailed(ctx, "key_not_found", reqInfo.IPAddress, reqInfo.UserAgent, nil)
			return nil, nil, nil, ErrInvalidAPIKey
		}
		log.With("error", err).Error("Failed to find API key")
		s.auditSvc.LogAPIKeyValidationFailed(ctx, "database_error", reqInfo.IPAddress, reqInfo.UserAgent, nil)
		return nil, nil, nil, fmt.Errorf("failed to find API key: %w", err)
	}
	// Always verify key hash first (before checking other conditions) to prevent timing attacks
	hashValid := s.VerifyAPIKey(key, apiKey.KeyHash)
	// Check if key is expired
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		log.With("api_key_id", apiKey.ID, "expired_at", apiKey.ExpiresAt).
			Debug("API key expired")
		s.auditSvc.LogAPIKeyExpired(ctx, apiKey.ID, apiKey.UserID, apiKey.OrgID, *apiKey.ExpiresAt)
		return nil, nil, nil, ErrAPIKeyExpired
	}
	// Check if key is active
	if apiKey.Status != StatusActive {
		log.With("api_key_id", apiKey.ID, "status", apiKey.Status).
			Debug("API key not active")
		s.auditSvc.LogAPIKeyValidationFailed(ctx, "key_not_active", reqInfo.IPAddress, reqInfo.UserAgent, &apiKey.ID)
		return nil, nil, nil, ErrInvalidAPIKey
	}
	// Check hash validity last to ensure constant time for all paths
	if !hashValid {
		log.With("api_key_id", apiKey.ID).Debug("API key hash verification failed")
		s.auditSvc.LogAPIKeyValidationFailed(ctx, "invalid_hash", reqInfo.IPAddress, reqInfo.UserAgent, &apiKey.ID)
		return nil, nil, nil, ErrInvalidAPIKey
	}
	// Update last used timestamp
	if err := s.apiKeyRepo.ValidateAndUpdateLastUsed(ctx, apiKey.OrgID, apiKey.ID); err != nil {
		log.With("api_key_id", apiKey.ID, "error", err).
			Error("Failed to update last used timestamp")
		return nil, nil, nil, err
	}
	// Retrieve user context
	usr, err := s.userRepo.GetByID(ctx, apiKey.UserID, apiKey.OrgID)
	if err != nil {
		log.With("user_id", apiKey.UserID, "error", err).Error("Failed to get user")
		return nil, nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	// Retrieve organization context
	organization, err := s.orgRepo.GetByID(ctx, apiKey.OrgID)
	if err != nil {
		log.With("org_id", apiKey.OrgID, "error", err).Error("Failed to get organization")
		return nil, nil, nil, fmt.Errorf("failed to get organization: %w", err)
	}
	// Log successful validation
	log.With(
		"api_key_id", apiKey.ID,
		"user_id", usr.ID,
		"org_id", organization.ID,
		"user_role", usr.Role,
	).Info("API key validated successfully")
	// Audit successful validation
	s.auditSvc.LogAPIKeyValidated(ctx, apiKey.ID, usr.ID, organization.ID)
	return apiKey, usr, organization, nil
}

// CreateAPIKey creates a new API key for a user
func (s *Service) CreateAPIKey(
	ctx context.Context,
	userID, orgID core.ID,
	name string,
	expiresAt *time.Time,
	rateLimitPerHour int,
) (*APIKey, string, error) {
	// Generate new API key
	key, err := s.GenerateAPIKey()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate API key: %w", err)
	}
	// Hash the key
	hash, err := s.HashAPIKey(key)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash API key: %w", err)
	}
	// Extract prefix for storage
	prefix := s.ExtractKeyPrefix(key)
	// Create API key entity
	id, err := core.NewID()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate ID: %w", err)
	}
	apiKey := &APIKey{
		ID:               id,
		UserID:           userID,
		OrgID:            orgID,
		KeyHash:          hash,
		KeyPrefix:        prefix,
		Name:             name,
		Status:           StatusActive,
		ExpiresAt:        expiresAt,
		RateLimitPerHour: rateLimitPerHour,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	// Store in repository
	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return nil, "", fmt.Errorf("failed to create API key: %w", err)
	}
	log := logger.FromContext(ctx)
	log.With(
		"api_key_id", apiKey.ID,
		"user_id", userID,
		"org_id", orgID,
		"name", name,
	).Info("API key created")
	// Audit key creation
	s.auditSvc.LogAPIKeyCreated(ctx, apiKey.ID, userID, orgID, name)
	return apiKey, key, nil
}

// RevokeAPIKey revokes an API key
func (s *Service) RevokeAPIKey(ctx context.Context, apiKeyID, orgID core.ID, revokedBy core.ID) error {
	apiKey, err := s.apiKeyRepo.GetByID(ctx, orgID, apiKeyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}
	apiKey.Status = StatusRevoked
	apiKey.UpdatedAt = time.Now()
	if err := s.apiKeyRepo.Update(ctx, apiKey); err != nil {
		return fmt.Errorf("failed to update API key: %w", err)
	}
	log := logger.FromContext(ctx)
	log.With("api_key_id", apiKeyID, "org_id", orgID).Info("API key revoked")
	// Audit key revocation
	s.auditSvc.LogAPIKeyRevoked(ctx, apiKeyID, apiKey.UserID, orgID, revokedBy)
	return nil
}
