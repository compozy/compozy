package daemon

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/telemetry"
)

const (
	// telemetryModelSnapshotTTL bounds how long a catalog change takes to reach exported model names.
	telemetryModelSnapshotTTL = 5 * time.Minute
	// telemetryModelMissRebuildAfter rebuilds sooner when a lookup misses, so a model the catalog learns after boot
	// (the first refresh usually lands after telemetry starts) exports its name within seconds, not minutes.
	telemetryModelMissRebuildAfter = 30 * time.Second
	telemetryModelListTimeout      = 10 * time.Second
)

// telemetryModelCatalog is the part of the model catalog runtime the classifier reads.
type telemetryModelCatalog interface {
	ListModels(ctx context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error)
}

// telemetryModelClasses classifies gen_ai.request.model values for the telemetry sanitizer: a model with a built-in or
// models.dev catalog row exports its name, a model known only from operator configuration exports "custom", and
// anything else "other". Lookups read an atomically swapped snapshot, so the export path never touches the catalog;
// a stale snapshot is rebuilt in the background by a worker the catalog runtime owns.
type telemetryModelClasses struct {
	catalog    telemetryModelCatalog
	begin      func() (complete func(), ok bool)
	logger     *slog.Logger
	now        func() time.Time
	snapshot   atomic.Pointer[telemetryModelSnapshot]
	rebuilding atomic.Bool
}

type telemetryModelSnapshot struct {
	classes map[string]telemetry.ModelClass
	builtAt time.Time
}

// newTelemetryModelClasses returns the classifier and starts its first build; until it lands, every model exports as
// "other", as the sanitizer requires for values not yet in a registry.
func newTelemetryModelClasses(runtime *modelCatalogRuntime, logger *slog.Logger) *telemetryModelClasses {
	if runtime == nil {
		return nil
	}
	classes := &telemetryModelClasses{
		catalog: runtime,
		begin:   runtime.workers.Begin,
		logger:  logger,
		now:     time.Now,
	}
	classes.refreshAsync()
	return classes
}

// Classify implements telemetry.Registries.Models.
func (c *telemetryModelClasses) Classify(provider, model string) telemetry.ModelClass {
	snapshot := c.snapshot.Load()
	if snapshot == nil {
		c.refreshAsync()
		return telemetry.ModelUnknown
	}
	class, known := snapshot.classes[telemetryModelKey(provider, model)]
	age := c.now().Sub(snapshot.builtAt)
	if age >= telemetryModelSnapshotTTL || (!known && age >= telemetryModelMissRebuildAfter) {
		c.refreshAsync()
	}
	return class
}

// refreshAsync rebuilds the snapshot once at a time on a worker joined at catalog shutdown.
func (c *telemetryModelClasses) refreshAsync() {
	if !c.rebuilding.CompareAndSwap(false, true) {
		return
	}
	complete, ok := c.begin()
	if !ok {
		c.rebuilding.Store(false)
		return
	}
	go func() {
		defer complete()
		defer c.rebuilding.Store(false)
		ctx, cancel := context.WithTimeout(context.Background(), telemetryModelListTimeout)
		defer cancel()
		if err := c.rebuild(ctx); err != nil && c.logger != nil {
			c.logger.WarnContext(ctx, "daemon: classify models for telemetry", "error", err)
		}
	}()
}

func (c *telemetryModelClasses) rebuild(ctx context.Context) error {
	models, err := c.catalog.ListModels(ctx, modelcatalog.ListOptions{IncludeAll: true, IncludeStale: true})
	if err != nil {
		return err
	}
	classes := make(map[string]telemetry.ModelClass, len(models))
	for i := range models {
		if class := telemetryModelClass(&models[i]); class != telemetry.ModelUnknown {
			classes[telemetryModelKey(models[i].ProviderID, models[i].ModelID)] = class
		}
	}
	c.snapshot.Store(&telemetryModelSnapshot{classes: classes, builtAt: c.now()})
	return nil
}

// telemetryModelClass trusts only the built-in and models.dev catalogs; operator-authored rows are "custom", and
// provider-live, extension, and session observations are user-influenced, so they stay unknown on their own.
func telemetryModelClass(model *modelcatalog.Model) telemetry.ModelClass {
	class := telemetry.ModelUnknown
	if model.Curated || model.ExplicitlyCurated {
		class = telemetry.ModelCustom
	}
	for _, source := range model.Sources {
		switch source.SourceKind {
		case modelcatalog.SourceKindBuiltin, modelcatalog.SourceKindModelsDev:
			return telemetry.ModelCatalog
		case modelcatalog.SourceKindConfig:
			class = telemetry.ModelCustom
		}
	}
	return class
}

func telemetryModelKey(provider, model string) string {
	return strings.TrimSpace(provider) + "\x00" + strings.TrimSpace(model)
}
