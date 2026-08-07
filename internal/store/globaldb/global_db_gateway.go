package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var (
	_ gateway.Store                 = (*GatewayRepo)(nil)
	_ gateway.ProviderIdentityStore = (*GatewayRepo)(nil)
)

// ProviderIdentity reads the operator-confirmed gateway provider tuple.
func (g *GatewayRepo) ProviderIdentity(ctx context.Context, name string) (gateway.ProviderIdentity, error) {
	if err := g.checkReady(ctx, "load gateway provider identity"); err != nil {
		return gateway.ProviderIdentity{}, err
	}
	row, err := g.queries.GetGatewayProvider(ctx, strings.TrimSpace(name))
	if errors.Is(err, sql.ErrNoRows) {
		return gateway.ProviderIdentity{}, gateway.ErrProviderNotFound
	}
	if err != nil {
		return gateway.ProviderIdentity{}, fmt.Errorf("store: load gateway provider identity: %w", err)
	}
	confirmedAt, err := parseNullableGatewayTime(row.ConfirmedAt)
	if err != nil {
		return gateway.ProviderIdentity{}, fmt.Errorf("store: parse gateway provider confirmation: %w", err)
	}
	return gateway.ProviderIdentity{
		Name: row.Name, InstallSource: row.InstallSource,
		DigestConfirmed: row.DigestConfirmed.String, ConfirmedAt: confirmedAt,
	}, nil
}

// Snapshot loads the complete operator-global gateway state.
func (g *GatewayRepo) Snapshot(ctx context.Context) (_ gateway.Snapshot, err error) {
	if err := g.checkReady(ctx, "load gateway snapshot"); err != nil {
		return gateway.Snapshot{}, err
	}
	tx, err := g.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return gateway.Snapshot{}, fmt.Errorf("store: begin gateway snapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway snapshot"))
		}
	}()
	queries := sqlcgen.New(tx)
	providers, err := queries.ListGatewayProviderActivations(ctx)
	if err != nil {
		return gateway.Snapshot{}, fmt.Errorf("store: list gateway provider activations: %w", err)
	}
	surfaces, err := queries.ListGatewaySurfaceExposure(ctx)
	if err != nil {
		return gateway.Snapshot{}, fmt.Errorf("store: list gateway surface exposure: %w", err)
	}
	devices, err := queries.ListActiveGatewayDeviceSessions(ctx)
	if err != nil {
		return gateway.Snapshot{}, fmt.Errorf("store: list active gateway devices: %w", err)
	}
	snapshot := gateway.Snapshot{
		Providers: make([]gateway.ProviderActivation, 0, len(providers)),
		Surfaces:  make([]gateway.SurfaceExposure, 0, len(surfaces)),
		Devices:   make([]gateway.DeviceSession, 0, len(devices)),
	}
	for _, row := range providers {
		mapped, mapErr := gatewayProviderActivationFromGenerated(row)
		if mapErr != nil {
			return gateway.Snapshot{}, mapErr
		}
		snapshot.Providers = append(snapshot.Providers, mapped)
	}
	for _, row := range surfaces {
		mapped, mapErr := gatewaySurfaceExposureFromGenerated(row)
		if mapErr != nil {
			return gateway.Snapshot{}, mapErr
		}
		snapshot.Surfaces = append(snapshot.Surfaces, mapped)
	}
	for _, row := range devices {
		mapped, mapErr := gatewayDeviceSessionFromGenerated(row)
		if mapErr != nil {
			return gateway.Snapshot{}, mapErr
		}
		snapshot.Devices = append(snapshot.Devices, mapped)
	}
	if err := tx.Commit(); err != nil {
		return gateway.Snapshot{}, fmt.Errorf("store: commit gateway snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

// Transition atomically writes desired state and advances its generation.
func (g *GatewayRepo) Transition(
	ctx context.Context,
	req gateway.TransitionRequest,
	now time.Time,
) (_ bool, err error) {
	if err := g.checkReady(ctx, "transition gateway exposure"); err != nil {
		return false, err
	}
	expected, err := gatewayGeneration(req.ExpectedGeneration)
	if err != nil {
		return false, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin gateway transition: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway transition"))
		}
	}()
	queries := sqlcgen.New(tx)
	var changed bool
	switch req.Target {
	case gateway.TargetProvider:
		changed, err = transitionGatewayProvider(ctx, queries, req, expected, now)
	case gateway.TargetSurface:
		changed, err = transitionGatewaySurface(ctx, queries, req, expected, now)
	default:
		err = fmt.Errorf("store: unsupported gateway transition target %q", req.Target)
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit gateway transition: %w", err)
	}
	committed = true
	return changed, nil
}

func transitionGatewayProvider(
	ctx context.Context,
	queries *sqlcgen.Queries,
	req gateway.TransitionRequest,
	expected int64,
	now time.Time,
) (bool, error) {
	current, err := queries.GetGatewayProviderActivation(ctx, sqlcgen.GetGatewayProviderActivationParams{
		ProviderName: req.Provider.Name,
		Tier:         string(req.Tier),
	})
	switch {
	case err == nil:
		if current.Generation != expected {
			return false, gateway.ErrGenerationConflict
		}
		if gateway.DesiredState(current.DesiredState) == req.Desired {
			return false, nil
		}
	case errors.Is(err, sql.ErrNoRows):
		if expected != 0 {
			return false, gateway.ErrGenerationConflict
		}
		if req.Desired == gateway.DesiredDisabled {
			return false, nil
		}
	default:
		return false, fmt.Errorf("store: read gateway provider activation: %w", err)
	}
	if req.Desired == gateway.DesiredEnabled {
		if err := queries.UpsertGatewayProvider(ctx, gatewayProviderParams(req.Provider, now)); err != nil {
			return false, fmt.Errorf("store: upsert gateway provider %q: %w", req.Provider.Name, err)
		}
	}
	_, err = queries.TransitionGatewayProviderActivation(ctx, sqlcgen.TransitionGatewayProviderActivationParams{
		ProviderName:       req.Provider.Name,
		Tier:               string(req.Tier),
		DesiredState:       string(req.Desired),
		ExpectedGeneration: expected,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, gateway.ErrGenerationConflict
		}
		if strings.Contains(err.Error(), "gateway_provider_activations.tier") {
			return false, gateway.ErrTierProviderConflict
		}
		return false, fmt.Errorf("store: transition gateway provider activation: %w", err)
	}
	return true, nil
}

func transitionGatewaySurface(
	ctx context.Context,
	queries *sqlcgen.Queries,
	req gateway.TransitionRequest,
	expected int64,
	now time.Time,
) (bool, error) {
	current, err := queries.GetGatewaySurfaceExposure(ctx, sqlcgen.GetGatewaySurfaceExposureParams{
		Surface: string(req.Surface),
		Tier:    string(req.Tier),
	})
	switch {
	case err == nil:
		if current.Generation != expected {
			return false, gateway.ErrGenerationConflict
		}
		if gateway.DesiredState(current.DesiredState) == req.Desired {
			return false, nil
		}
	case errors.Is(err, sql.ErrNoRows):
		if expected != 0 {
			return false, gateway.ErrGenerationConflict
		}
		if req.Desired == gateway.DesiredDisabled {
			return false, nil
		}
	default:
		return false, fmt.Errorf("store: read gateway surface exposure: %w", err)
	}
	var consentedAt sql.NullString
	if req.Consent && req.Desired == gateway.DesiredEnabled {
		consentedAt = sql.NullString{String: store.FormatTimestamp(now), Valid: true}
	}
	_, err = queries.TransitionGatewaySurfaceExposure(ctx, sqlcgen.TransitionGatewaySurfaceExposureParams{
		Surface:            string(req.Surface),
		Tier:               string(req.Tier),
		DesiredState:       string(req.Desired),
		ConsentedAt:        consentedAt,
		ExpectedGeneration: expected,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, gateway.ErrGenerationConflict
		}
		return false, fmt.Errorf("store: transition gateway surface exposure: %w", err)
	}
	return true, nil
}

// DisableAll atomically clears desired and observed exposure for every tier.
func (g *GatewayRepo) DisableAll(ctx context.Context) (_ bool, err error) {
	if err := g.checkReady(ctx, "disable all gateway exposure"); err != nil {
		return false, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin disable-all gateway transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "disable-all gateway"))
		}
	}()
	queries := sqlcgen.New(tx)
	providers, err := queries.DisableAllGatewayProviderActivations(ctx)
	if err != nil {
		return false, fmt.Errorf("store: disable gateway providers: %w", err)
	}
	surfaces, err := queries.DisableAllGatewaySurfaceExposure(ctx)
	if err != nil {
		return false, fmt.Errorf("store: disable gateway surfaces: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit disable-all gateway transaction: %w", err)
	}
	committed = true
	return providers+surfaces > 0, nil
}

// SetObserved atomically applies effect output only to the expected generations.
func (g *GatewayRepo) SetObserved(
	ctx context.Context,
	plan gateway.TierPlan,
	providerObserved gateway.ProviderObservedState,
	surfaceObserved gateway.SurfaceObservedState,
	at time.Time,
	cause string,
) (_ bool, err error) {
	if err := g.checkReady(ctx, "set gateway provider observed state"); err != nil {
		return false, err
	}
	if plan.Provider == nil {
		return false, errors.New("store: gateway provider plan is required")
	}
	generation, err := gatewayGeneration(plan.Provider.Generation)
	if err != nil {
		return false, err
	}
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("store: begin gateway observed-state transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			err = errors.Join(err, rollbackTx(tx, "gateway observed-state"))
		}
	}()
	queries := sqlcgen.New(tx)
	rows, err := queries.UpdateGatewayProviderObserved(ctx, sqlcgen.UpdateGatewayProviderObservedParams{
		ObservedState: string(providerObserved), LastHealthAt: nullableGatewayTime(at),
		LastError: store.SQLNullString(cause), ProviderName: plan.Provider.ProviderName,
		Tier: string(plan.Provider.Tier), ExpectedGeneration: generation,
	})
	if err != nil {
		return false, fmt.Errorf("store: set gateway provider observed state: %w", err)
	}
	if rows != 1 {
		return false, nil
	}
	for _, exposure := range plan.Surfaces {
		surfaceGeneration, generationErr := gatewayGeneration(exposure.Generation)
		if generationErr != nil {
			return false, generationErr
		}
		rows, updateErr := queries.UpdateGatewaySurfaceObserved(ctx, sqlcgen.UpdateGatewaySurfaceObservedParams{
			ObservedState: string(surfaceObserved), Surface: string(exposure.Surface),
			Tier: string(exposure.Tier), ExpectedGeneration: surfaceGeneration,
		})
		if updateErr != nil {
			return false, fmt.Errorf("store: set gateway surface observed state: %w", updateErr)
		}
		if rows != 1 {
			return false, nil
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("store: commit gateway observed-state transaction: %w", err)
	}
	committed = true
	return true, nil
}

func gatewayGeneration(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, fmt.Errorf("store: gateway generation exceeds SQLite integer range: %d", value)
	}
	return int64(value), nil
}
