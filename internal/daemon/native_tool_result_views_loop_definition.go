package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/loop/dsl"
)

type compactLoopDefinition struct {
	Name        string              `json:"name"`
	Version     int                 `json:"version"`
	Description string              `json:"description,omitempty"`
	Source      contract.LoopSource `json:"source"`
}

func projectCompactLoopDefinition(p *contract.LoopDefinitionPayload) *compactLoopDefinition {
	if p == nil {
		return nil
	}
	result := &compactLoopDefinition{
		Name:        p.Name,
		Version:     p.Version,
		Description: p.Description,
		Source:      p.Source,
	}
	return result
}

type compactLoopCreate struct {
	Loop compactLoopDefinition `json:"loop"`
}

func projectCompactLoopCreate(p *contract.LoopResponse) *compactLoopCreate {
	if p == nil {
		return nil
	}
	result := &compactLoopCreate{}
	result.Loop = *projectCompactLoopDefinition(&p.Loop)
	return result
}

func viewLoopCreate(p *contract.LoopResponse) any { return projectCompactLoopCreate(p) }

type compactLoopConfigure struct {
	Config *contract.LoopConfig `json:"config"`
}

func projectCompactLoopConfigure(p *contract.LoopConfigResponse) *compactLoopConfigure {
	if p == nil {
		return nil
	}
	result := &compactLoopConfigure{
		Config: p.Config,
	}
	return result
}

func viewLoopConfigure(p contract.LoopConfigResponse) any { return projectCompactLoopConfigure(&p) }

type compactLoopInspect struct {
	Name     string               `json:"name"`
	Inputs   map[string]dsl.Input `json:"inputs"`
	Contract dsl.Contract         `json:"contract"`
	Start    []dsl.StartBinding   `json:"start"`
	Version  int                  `json:"version"`
}

func projectCompactLoopInspect(p *loopInspectResult) *compactLoopInspect {
	if p == nil {
		return nil
	}
	result := &compactLoopInspect{
		Name:     p.Name,
		Inputs:   p.Inputs,
		Contract: p.Contract,
		Start:    p.Start,
		Version:  p.Version,
	}
	return result
}

func viewLoopInspect(p loopInspectResult) any { return projectCompactLoopInspect(&p) }

type loopInspectResult struct {
	Name            string                        `json:"name"`
	Inputs          map[string]dsl.Input          `json:"inputs"`
	Contract        dsl.Contract                  `json:"contract"`
	EffectiveConfig *contract.LoopEffectiveConfig `json:"effective_config"`
	Start           []dsl.StartBinding            `json:"start"`
	Version         int                           `json:"version"`
}

type compactLoopCatalogEntry struct {
	WorkspaceID   string                               `json:"workspace_id,omitempty"`
	Name          string                               `json:"name"`
	Version       int                                  `json:"version"`
	Description   string                               `json:"description,omitempty"`
	Source        contract.LoopSource                  `json:"source"`
	LastRun       *contract.LoopCatalogLastRunPayload  `json:"last_run,omitzero"`
	Aggregate30d  contract.LoopCatalogAggregatePayload `json:"aggregate_30d"`
	SuccessRate30 float64                              `json:"success_rate_30d"`
}

func projectCompactLoopCatalogEntry(p *contract.LoopCatalogEntryPayload) *compactLoopCatalogEntry {
	if p == nil {
		return nil
	}
	result := &compactLoopCatalogEntry{
		WorkspaceID:   p.WorkspaceID,
		Name:          p.Name,
		Version:       p.Version,
		Description:   p.Description,
		Source:        p.Source,
		LastRun:       p.LastRun,
		Aggregate30d:  p.Aggregate30d,
		SuccessRate30: p.SuccessRate30,
	}
	return result
}

type compactLoopList struct {
	Loops  []compactLoopCatalogEntry         `json:"loops"`
	Facets contract.LoopCatalogFacetsPayload `json:"facets"`
	Page   contract.CountedCursorPagePayload `json:"page"`
}

func projectCompactLoopList(p *contract.LoopsResponse) *compactLoopList {
	if p == nil {
		return nil
	}
	result := &compactLoopList{
		Facets: p.Facets,
		Page:   p.Page,
	}
	if p.Loops != nil {
		result.Loops = make([]compactLoopCatalogEntry, len(p.Loops))
		for i := range p.Loops {
			result.Loops[i] = *projectCompactLoopCatalogEntry(&p.Loops[i])
		}
	}
	return result
}

func viewLoopList(p contract.LoopsResponse) any { return projectCompactLoopList(&p) }
