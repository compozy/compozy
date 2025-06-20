package org

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/sethvargo/go-retry"
)

// CreateOrganizationRequest represents the request to create an organization
type CreateOrganizationRequest struct {
	Name       string `json:"name"                  validate:"required,min=3,max=255"`
	AdminName  string `json:"admin_name,omitempty"`
	AdminEmail string `json:"admin_email,omitempty"`
}

// TemporalService defines the interface for Temporal namespace operations
type TemporalService interface {
	// ProvisionNamespace creates a new Temporal namespace
	ProvisionNamespace(ctx context.Context, namespace string) error
	// DeleteNamespace removes a Temporal namespace
	DeleteNamespace(ctx context.Context, namespace string) error
	// NamespaceExists checks if a namespace exists
	NamespaceExists(ctx context.Context, namespace string) (bool, error)
}

// Config holds configuration for the organization service
type Config struct {
	// Retry settings for namespace provisioning
	RetryAttempts   int           `json:"retry_attempts"`
	RetryDelayStart time.Duration `json:"retry_delay_start"`
	RetryDelayMax   time.Duration `json:"retry_delay_max"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		RetryAttempts:   3,
		RetryDelayStart: 500 * time.Millisecond,
		RetryDelayMax:   5 * time.Second,
	}
}

// Service provides organization lifecycle management with Temporal namespace provisioning
type Service struct {
	repo            Repository
	temporalService TemporalService
	db              store.DBInterface
	config          *Config
}

// NewService creates a new organization service instance
func NewService(
	repo Repository,
	temporalService TemporalService,
	db store.DBInterface,
	config *Config,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	return &Service{
		repo:            repo,
		temporalService: temporalService,
		db:              db,
		config:          config,
	}
}

// CreateOrganization creates a new organization with atomic database + Temporal namespace creation
func (s *Service) CreateOrganization(ctx context.Context, req *CreateOrganizationRequest) (*Organization, error) {
	log := logger.FromContext(ctx)

	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Check for existing organization with same name
	existingOrg, err := s.repo.GetByName(ctx, req.Name)
	if err != nil && err != ErrOrganizationNotFound {
		return nil, fmt.Errorf("failed to check organization uniqueness: %w", err)
	}
	if existingOrg != nil {
		return nil, fmt.Errorf("organization with name '%s' already exists", req.Name)
	}

	// Create organization entity
	org, err := NewOrganization(req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization entity: %w", err)
	}

	// Generate namespace with proper format: org-{org-slug}-{short-uuid}
	namespace := s.generateNamespaceWithUUID(req.Name, org.ID)
	org.TemporalNamespace = namespace

	log.With(
		"org_id", org.ID,
		"org_name", org.Name,
		"namespace", org.TemporalNamespace,
	).Info("Creating organization")

	// Step 1: Create organization in database with provisioning status
	err = s.withTransaction(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		return txRepo.Create(txCtx, org)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create organization in database: %w", err)
	}

	log.With("org_id", org.ID).Info("Organization created in database, provisioning Temporal namespace")

	// Step 2: Check if namespace already exists (idempotency for crash recovery)
	exists, err := s.temporalService.NamespaceExists(ctx, org.TemporalNamespace)
	if err != nil {
		log.With("org_id", org.ID, "error", err).
			Warn("Failed to check namespace existence, proceeding with provisioning")
		exists = false
	}

	// Step 3: Provision Temporal namespace (outside transaction) if it doesn't exist
	if !exists {
		if err := s.provisionTemporalNamespaceWithRetry(ctx, org.TemporalNamespace); err != nil {
			log.With("org_id", org.ID, "error", err).
				Error("Failed to provision Temporal namespace, marking organization as failed")
			if updateErr := s.updateOrganizationStatus(ctx, org.ID, StatusProvisioningFailed); updateErr != nil {
				log.With("org_id", org.ID, "error", updateErr).Error("Failed to update organization status to failed")
			}
			return nil, fmt.Errorf("failed to provision Temporal namespace: %w", err)
		}
	} else {
		log.With("org_id", org.ID, "namespace", org.TemporalNamespace).
			Info("Temporal namespace already exists, proceeding to activation")
	}

	log.With("org_id", org.ID).Info("Temporal namespace provisioned successfully, activating organization")

	// Step 4: Update status to active
	if err := s.updateOrganizationStatus(ctx, org.ID, StatusActive); err != nil {
		log.With("org_id", org.ID, "error", err).Error("Failed to activate organization")
		return nil, fmt.Errorf("failed to activate organization: %w", err)
	}

	// Update local entity status
	org.Status = StatusActive
	org.UpdatedAt = time.Now().UTC()

	log.With("org_id", org.ID, "namespace", org.TemporalNamespace).Info("Organization creation completed successfully")

	return org, nil
}

// GetOrganization retrieves an organization by ID
func (s *Service) GetOrganization(ctx context.Context, id core.ID) (*Organization, error) {
	return s.repo.GetByID(ctx, id)
}

// GetOrganizationByName retrieves an organization by name
func (s *Service) GetOrganizationByName(ctx context.Context, name string) (*Organization, error) {
	return s.repo.GetByName(ctx, name)
}

// UpdateOrganizationStatus updates the status of an organization
func (s *Service) UpdateOrganizationStatus(ctx context.Context, id core.ID, status OrganizationStatus) error {
	return s.updateOrganizationStatus(ctx, id, status)
}

// ListOrganizations retrieves organizations with pagination
func (s *Service) ListOrganizations(ctx context.Context, limit, offset int) ([]*Organization, error) {
	return s.repo.List(ctx, limit, offset)
}

// validateCreateRequest validates the create organization request
func (s *Service) validateCreateRequest(req *CreateOrganizationRequest) error {
	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}
	return ValidateOrganizationName(req.Name)
}

var (
	// invalidNamespaceCharsRegex matches any character that is not alphanumeric or hyphen
	invalidNamespaceCharsRegex = regexp.MustCompile(`[^a-zA-Z0-9-]+`)
)

// generateNamespaceWithUUID generates a Temporal namespace with UUID suffix
func (s *Service) generateNamespaceWithUUID(orgName string, orgID core.ID) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(orgName)
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")
	// Remove all invalid characters (keep only alphanumeric and hyphens)
	slug = invalidNamespaceCharsRegex.ReplaceAllString(slug, "")
	// Remove any consecutive hyphens
	slug = consecutiveHyphensRegex.ReplaceAllString(slug, "-")
	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")
	// Ensure the namespace starts with a letter
	if slug != "" && !startsWithLetterRegex.MatchString(slug) {
		slug = "org-" + slug
	}
	// Get short UUID with length check to prevent panic
	orgIDStr := string(orgID)
	shortUUID := orgIDStr
	if len(orgIDStr) > 8 {
		shortUUID = orgIDStr[:8]
	}
	// Format: org-{org-slug}-{short-uuid}
	return fmt.Sprintf("org-%s-%s", slug, shortUUID)
}

// provisionTemporalNamespaceWithRetry provisions a Temporal namespace with retry logic using go-retry
func (s *Service) provisionTemporalNamespaceWithRetry(ctx context.Context, namespace string) error {
	log := logger.FromContext(ctx)
	// Use retry with exponential backoff, cap, and jitter
	backoff := retry.NewExponential(s.config.RetryDelayStart)
	backoff = retry.WithCappedDuration(s.config.RetryDelayMax, backoff)     // Cap max delay
	backoff = retry.WithJitter(100*time.Millisecond, backoff)               // Add jitter
	backoff = retry.WithMaxRetries(uint64(s.config.RetryAttempts), backoff) // #nosec G115
	return retry.Do(ctx, backoff, func(ctx context.Context) error {
		log.With("namespace", namespace).Debug("Attempting to provision Temporal namespace")
		err := s.temporalService.ProvisionNamespace(ctx, namespace)
		if err != nil {
			log.With("namespace", namespace, "error", err).
				Warn("Failed to provision Temporal namespace, will retry")
			return retry.RetryableError(err)
		}
		log.With("namespace", namespace).Info("Temporal namespace provisioned successfully")
		return nil
	},
	)
}

// updateOrganizationStatus updates the status of an organization
func (s *Service) updateOrganizationStatus(ctx context.Context, id core.ID, status OrganizationStatus) error {
	return s.withTransaction(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		txRepo := s.repo.WithTx(tx)
		return txRepo.UpdateStatus(txCtx, id, status)
	})
}

// withTransaction executes a function within a transaction with proper context management
func (s *Service) withTransaction(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	log := logger.FromContext(ctx)
	txCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	tx, err := s.db.Begin(txCtx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(txCtx); rbErr != nil {
				log.With("error", rbErr).Error("Failed to rollback transaction after panic")
			}
			panic(p)
		} else if err != nil {
			if rbErr := tx.Rollback(txCtx); rbErr != nil {
				err = fmt.Errorf("tx error: %v, rollback error: %w", err, rbErr)
				log.With("error", rbErr).Error("Failed to rollback transaction")
			}
		} else {
			if commitErr := tx.Commit(txCtx); commitErr != nil {
				err = fmt.Errorf("failed to commit transaction: %w", commitErr)
				log.With("error", commitErr).Error("Failed to commit transaction")
			}
		}
	}()
	err = fn(txCtx, tx)
	return err
}
