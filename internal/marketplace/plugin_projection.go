package marketplace

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const (
	maxPluginResolves   = 4
	maxProjectedPlugins = 200
	pluginRefreshBudget = 10 * time.Minute
	budgetExhausted     = "refresh_budget_exhausted"
)

var ErrRefreshBudgetExhausted = errors.New(budgetExhausted)

type PluginContents struct {
	Skills     int `json:"skills"`
	MCPServers int `json:"mcp_servers"`
	Hooks      int `json:"hooks"`
	Loops      int `json:"loops"`
	Agents     int `json:"agents"`
	Bridges    int `json:"bridges"`
}

type PluginInspection struct {
	InstanceName string
	Inputs       []EntryInput
	Contents     PluginContents
}

type PluginResolver interface {
	Resolve(
		context.Context,
		pluginsource.Document,
		*pluginsource.Snapshot,
		pluginsource.Plugin,
	) (pluginsource.AcquisitionRecord, error)
	Inspect(context.Context, string, func(context.Context, string) error) error
}

var _ PluginResolver = (*pluginsource.Resolver)(nil)

type PluginInspector func(context.Context, string) (PluginInspection, error)

type PluginProjector struct {
	resolver PluginResolver
	inspect  PluginInspector
	budget   time.Duration
}

type PluginProjectorOption func(*PluginProjector)

// WithPluginRefreshBudget can shorten, but cannot exceed, the source refresh deadline.
func WithPluginRefreshBudget(budget time.Duration) PluginProjectorOption {
	return func(projector *PluginProjector) {
		if budget > 0 && budget < pluginRefreshBudget {
			projector.budget = budget
		}
	}
}

func NewPluginProjector(
	resolver PluginResolver,
	inspect PluginInspector,
	options ...PluginProjectorOption,
) (*PluginProjector, error) {
	if resolver == nil || inspect == nil {
		return nil, errors.New("marketplace: plugin resolver and install loader are required")
	}
	projector := &PluginProjector{resolver: resolver, inspect: inspect, budget: pluginRefreshBudget}
	for _, option := range options {
		if option != nil {
			option(projector)
		}
	}
	return projector, nil
}

type pluginEntry struct {
	InstanceName string `json:"instance_name,omitempty"`
	extensionEntry
	SourceRef   string                          `json:"source_ref,omitempty"`
	Homepage    string                          `json:"homepage,omitempty"`
	License     string                          `json:"license,omitempty"`
	Category    string                          `json:"category,omitempty"`
	Keywords    []string                        `json:"keywords,omitempty"`
	Acquisition *pluginsource.AcquisitionRecord `json:"acquisition,omitempty"`
	Contents    PluginContents                  `json:"contents"`
}

type projectedPlugin struct {
	entry       Entry
	diagnostics []pluginsource.Diagnostic
	drop        bool
	err         error
}

// Project joins every bounded worker before the caller may release the shared snapshot.
func (p *PluginProjector) Project(
	ctx context.Context,
	doc pluginsource.Document,
	name string,
	snapshot *pluginsource.Snapshot,
) ([]Entry, []pluginsource.Diagnostic, error) {
	if ctx == nil || !entryIDPattern.MatchString(name) || name == "." || name == ".." {
		return nil, nil, errors.New("marketplace: context and source name are required")
	}
	workCtx, cancel := context.WithTimeout(ctx, p.budget)
	defer cancel()
	return p.project(ctx, workCtx, doc, name, snapshot)
}

func (p *PluginProjector) project(
	ctx, workCtx context.Context,
	doc pluginsource.Document,
	name string,
	snapshot *pluginsource.Snapshot,
) ([]Entry, []pluginsource.Diagnostic, error) {
	results := make([]projectedPlugin, len(doc.Plugins))
	for index, plugin := range doc.Plugins {
		results[index] = p.projectBlocked(
			plugin,
			name,
			doc.SourceRef,
			budgetExhausted,
			"source refresh budget was exhausted",
		)
	}
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(maxPluginResolves, len(doc.Plugins)) {
		workers.Go(func() {
			for index := range jobs {
				if workCtx.Err() == nil {
					results[index] = p.projectOne(workCtx, doc, name, snapshot, doc.Plugins[index])
				}
			}
		})
	}
	for index := range min(len(doc.Plugins), maxProjectedPlugins) {
		select {
		case jobs <- index:
		case <-workCtx.Done():
		}
		if workCtx.Err() != nil {
			break
		}
	}
	close(jobs)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	entries := make([]Entry, 0, len(results))
	diagnostics := append([]pluginsource.Diagnostic(nil), doc.Diagnostics...)
	exhausted := workCtx.Err() != nil
	for _, result := range results {
		if result.err != nil {
			return nil, nil, result.err
		}
		diagnostics = append(diagnostics, result.diagnostics...)
		if !result.drop {
			entries = append(entries, result.entry)
		}
		exhausted = exhausted || result.entry.InstallBlocker == budgetExhausted
	}
	if exhausted {
		diagnostics = append(
			diagnostics,
			pluginsource.Diagnostic{Code: budgetExhausted, Message: "source refresh budget was exhausted"},
		)
	}
	return entries, diagnostics, nil
}

func (p *PluginProjector) projectOne(
	ctx context.Context,
	doc pluginsource.Document,
	name string,
	snapshot *pluginsource.Snapshot,
	plugin pluginsource.Plugin,
) projectedPlugin {
	record, err := p.resolver.Resolve(ctx, doc, snapshot, plugin)
	if err != nil {
		return p.projectError(plugin, name, doc.SourceRef, err)
	}
	var inspection PluginInspection
	err = p.resolver.Inspect(ctx, record.DigestSHA256, func(ctx context.Context, root string) error {
		var loadErr error
		inspection, loadErr = p.inspect(ctx, root)
		return loadErr
	})
	result := p.projectError(plugin, name, doc.SourceRef, err)
	if result.err != nil {
		return result
	}
	value := pluginPayload(plugin, name, doc.SourceRef)
	value.Acquisition, value.Contents, value.Inputs = &record, inspection.Contents, inspection.Inputs
	value.InstanceName = inspection.InstanceName
	value.Version, value.DigestSHA256 = record.Version, record.DigestSHA256
	entry, encodeErr := commonEntry(value.entryCommon, value)
	if encodeErr != nil {
		return projectedPlugin{err: encodeErr}
	}
	entry.SourceName, entry.InstallSlug, entry.Tier = name, value.InstallSlug, extensionTierUnverified
	entry.Layout, entry.ResolvedRef, entry.DigestSHA256 = record.Layout, record.ResolvedRef, record.DigestSHA256
	entry.Inputs, entry.Icon = inspection.Inputs, value.Icon
	entry.Installable, entry.InstallBlocker = err == nil, result.entry.InstallBlocker
	result.entry = entry
	return result
}

func (p *PluginProjector) projectError(plugin pluginsource.Plugin, name, sourceRef string, err error) projectedPlugin {
	if err == nil {
		return projectedPlugin{}
	}
	code := "load_failed"
	switch {
	case errors.Is(err, pluginsource.ErrSourceOutsideCheckout):
		return projectedPlugin{
			drop: true,
			diagnostics: []pluginsource.Diagnostic{
				{Plugin: plugin.Name, Code: "source_outside_checkout", Message: err.Error()},
			},
		}
	case errors.Is(err, context.DeadlineExceeded):
		code = budgetExhausted
	case errors.Is(err, pluginsource.ErrPackageUnavailable):
		code = "package_unavailable"
	case errors.Is(err, pluginsource.ErrSourceUnreachable):
		code = "source_unreachable"
	}
	return p.projectBlocked(plugin, name, sourceRef, code, err.Error())
}

func (p *PluginProjector) projectBlocked(
	plugin pluginsource.Plugin, name, sourceRef, code, message string,
) projectedPlugin {
	value := pluginPayload(plugin, name, sourceRef)
	entry, err := commonEntry(value.entryCommon, value)
	entry.SourceName, entry.InstallSlug = name, value.InstallSlug
	entry.Tier, entry.Icon = extensionTierUnverified, value.Icon
	entry.InstallBlocker = code
	return projectedPlugin{
		entry:       entry,
		err:         err,
		diagnostics: []pluginsource.Diagnostic{{Plugin: plugin.Name, Code: code, Message: message}},
	}
}

func pluginPayload(plugin pluginsource.Plugin, name, sourceRef string) pluginEntry {
	icon := plugin.Icon
	if ValidateIcon(icon) != nil {
		icon = ""
	}
	return pluginEntry{SourceRef: sourceRef, Homepage: plugin.Homepage, License: plugin.License,
		Category: plugin.Category, Keywords: plugin.Keywords, extensionEntry: extensionEntry{
			entryCommon: entryCommon{
				EntryID:     plugin.Name,
				Name:        plugin.Name,
				Description: plugin.Description,
				Version:     plugin.Version,
			},
			Icon:        icon,
			InstallSlug: (Origin{SourceRef: sourceRef, EntryID: plugin.Name}).InstallSlug(name),
			Tier:        extensionTierUnverified,
			Author: strings.TrimSpace(
				plugin.Author,
			),
			Repository: strings.TrimSpace(plugin.Repository),
			Format:     ExtensionFormatAgentPlugin,
		}}
}
