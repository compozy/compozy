package globaldb

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	automation "github.com/compozy/agh/internal/automation/model"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

const automationJobRichSelectSQL = `SELECT
	id, scope, name, agent_name, workspace_id, prompt, schedule, task,
	enabled, retry, fire_limit, source, target_kind, loop_workspace_id,
	loop_name, loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
	FROM automation_jobs`

const automationTriggerRichSelectSQL = `SELECT
	id, scope, name, agent_name, workspace_id, prompt, event, filter,
	enabled, retry, fire_limit, source, webhook_id, endpoint_slug,
	webhook_secret_ref, target_kind, loop_workspace_id, loop_name,
	loop_inputs, loop_input_mapping, loop_network_participation, created_at, updated_at
	FROM automation_triggers`

func upsertAutomationJobCatalog(ctx context.Context, exec globalSQLExecutor, job automation.Job) error {
	search := automationJobCatalogSearch(job)
	loopName := automationJobCatalogLoopName(job)
	if err := sqlcgen.New(exec).UpsertAutomationJobCatalog(ctx, sqlcgen.UpsertAutomationJobCatalogParams{
		JobID: job.ID, Scope: string(job.Scope), WorkspaceID: strings.TrimSpace(job.WorkspaceID),
		Source: string(job.Source), SourceRank: int64(automation.ListSourceRank(job.Source)),
		Name: job.Name, LoopName: loopName, Enabled: job.Enabled, SearchName: search.name,
		SearchAgentName: search.agentName, SearchPrompt: search.prompt, SearchScope: search.scope,
		SearchSource: search.source, SearchScheduleMode: search.scheduleMode,
		SearchScheduleExpr: search.scheduleExpr, SearchScheduleInterval: search.scheduleInterval,
		SearchScheduleTime: search.scheduleTime,
	}); err != nil {
		return fmt.Errorf("store: upsert automation job catalog %q: %w", job.ID, err)
	}
	return nil
}

func upsertAutomationTriggerCatalog(ctx context.Context, exec globalSQLExecutor, trigger automation.Trigger) error {
	search := automationTriggerCatalogSearch(trigger)
	loopName := automationTriggerCatalogLoopName(trigger)
	queries := sqlcgen.New(exec)
	if err := queries.UpsertAutomationTriggerCatalog(ctx, sqlcgen.UpsertAutomationTriggerCatalogParams{
		TriggerID:          trigger.ID,
		Scope:              string(trigger.Scope),
		WorkspaceID:        strings.TrimSpace(trigger.WorkspaceID),
		Event:              trigger.Event,
		Source:             string(trigger.Source),
		SourceRank:         int64(automation.ListSourceRank(trigger.Source)),
		Name:               trigger.Name,
		LoopName:           loopName,
		Enabled:            trigger.Enabled,
		SearchName:         search.name,
		SearchAgentName:    search.agentName,
		SearchPrompt:       search.prompt,
		SearchScope:        search.scope,
		SearchSource:       search.source,
		SearchEvent:        search.event,
		SearchEndpointSlug: search.endpointSlug,
		SearchWebhookID:    search.webhookID,
	}); err != nil {
		return fmt.Errorf("store: upsert automation trigger catalog %q: %w", trigger.ID, err)
	}
	if err := queries.DeleteAutomationTriggerCatalogTerms(ctx, trigger.ID); err != nil {
		return fmt.Errorf("store: replace automation trigger catalog terms %q: %w", trigger.ID, err)
	}
	termsJSON, err := json.Marshal(automationTriggerFilterTerms(trigger.Filter))
	if err != nil {
		return fmt.Errorf("store: encode automation trigger catalog terms %q: %w", trigger.ID, err)
	}
	if err := queries.InsertAutomationTriggerCatalogTerms(ctx, sqlcgen.InsertAutomationTriggerCatalogTermsParams{
		TriggerID: trigger.ID, TermsJson: string(termsJSON),
	}); err != nil {
		return fmt.Errorf("store: insert automation trigger catalog terms %q: %w", trigger.ID, err)
	}
	return nil
}

type automationJobCatalogSearchFields struct {
	name             string
	agentName        string
	prompt           string
	scope            string
	source           string
	scheduleMode     string
	scheduleExpr     string
	scheduleInterval string
	scheduleTime     string
}

func automationJobCatalogSearch(job automation.Job) automationJobCatalogSearchFields {
	search := automationJobCatalogSearchFields{
		name:      strings.ToLower(job.Name),
		agentName: strings.ToLower(job.AgentName),
		prompt:    strings.ToLower(job.Prompt),
		scope:     strings.ToLower(string(job.Scope)),
		source:    strings.ToLower(string(job.Source)),
	}
	if job.Schedule != nil {
		search.scheduleMode = strings.ToLower(string(job.Schedule.Mode))
		search.scheduleExpr = strings.ToLower(job.Schedule.Expr)
		search.scheduleInterval = strings.ToLower(job.Schedule.Interval)
		search.scheduleTime = strings.ToLower(job.Schedule.Time)
	}
	return search
}

type automationTriggerCatalogSearchFields struct {
	name         string
	agentName    string
	prompt       string
	scope        string
	source       string
	event        string
	endpointSlug string
	webhookID    string
}

func automationTriggerCatalogSearch(trigger automation.Trigger) automationTriggerCatalogSearchFields {
	return automationTriggerCatalogSearchFields{
		name:         strings.ToLower(trigger.Name),
		agentName:    strings.ToLower(trigger.AgentName),
		prompt:       strings.ToLower(trigger.Prompt),
		scope:        strings.ToLower(string(trigger.Scope)),
		source:       strings.ToLower(string(trigger.Source)),
		event:        strings.ToLower(trigger.Event),
		endpointSlug: strings.ToLower(trigger.EndpointSlug),
		webhookID:    strings.ToLower(trigger.WebhookID),
	}
}

func automationTriggerFilterTerms(filter map[string]string) []string {
	terms := make(map[string]struct{}, len(filter)*2)
	for key, value := range filter {
		terms[strings.ToLower(key)] = struct{}{}
		terms[strings.ToLower(value)] = struct{}{}
	}
	result := make([]string, 0, len(terms))
	for term := range terms {
		result = append(result, term)
	}
	slices.Sort(result)
	return result
}

func automationJobCatalogLoopName(job automation.Job) string {
	if !job.IsLoopTarget() || job.LoopTarget == nil {
		return ""
	}
	return strings.TrimSpace(job.LoopTarget.LoopName)
}

func automationTriggerCatalogLoopName(trigger automation.Trigger) string {
	if !trigger.IsLoopTarget() || trigger.LoopTarget == nil {
		return ""
	}
	return strings.TrimSpace(trigger.LoopTarget.LoopName)
}
