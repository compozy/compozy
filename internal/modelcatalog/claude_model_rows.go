package modelcatalog

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

type claudeModelCandidate struct {
	id          string
	displayName string
	family      string
	version     []int
}

func parseClaudeModelRows(
	providerID string,
	models compozyconfig.ProviderModelsConfig,
	modelOption acp.SessionConfigOption,
	nowTime time.Time,
) []ModelRow {
	candidates := claudeModelCandidates(models)
	byID := make(map[string]int, len(modelOption.Values))
	rows := make([]ModelRow, 0, len(modelOption.Values))
	seenTransportIDs := make(map[string]struct{}, len(modelOption.Values))
	for _, value := range modelOption.Values {
		transportModelID := strings.TrimSpace(value.Value)
		if transportModelID == "" {
			continue
		}
		if _, exists := seenTransportIDs[transportModelID]; exists {
			continue
		}
		seenTransportIDs[transportModelID] = struct{}{}

		modelID := claudeLogicalModelID(transportModelID, value.Label, models, candidates)
		if modelID == "" {
			modelID = transportModelID
		}
		binding := ModelTransportBinding{
			TransportModelID: transportModelID,
			Label:            strings.TrimSpace(value.Label),
		}
		if index, exists := byID[modelID]; exists {
			rows[index].TransportBindings = appendTransportBinding(rows[index].TransportBindings, binding)
			continue
		}

		available := true
		row := ModelRow{
			ProviderID:        strings.TrimSpace(providerID),
			ModelID:           modelID,
			DisplayName:       claudeLiveDisplayName(modelID, value.Label, candidates),
			SourceID:          SourceKindProviderLiveID(providerID),
			SourceKind:        SourceKindProviderLive,
			Priority:          PriorityProviderLive,
			Available:         &available,
			TransportBindings: []ModelTransportBinding{binding},
			RefreshedAt:       nowTime,
		}
		byID[modelID] = len(rows)
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(left ModelRow, right ModelRow) int {
		return cmp.Compare(left.ModelID, right.ModelID)
	})
	return rows
}

func claudeModelCandidates(models compozyconfig.ProviderModelsConfig) []claudeModelCandidate {
	candidates := make([]claudeModelCandidate, 0, len(models.Curated))
	for _, model := range models.Curated {
		modelID := strings.TrimSpace(model.ID)
		if modelID == "" {
			continue
		}
		candidates = append(candidates, claudeModelCandidate{
			id:          modelID,
			displayName: strings.TrimSpace(model.DisplayName),
			family:      claudeModelFamily(modelID),
			version:     claudeModelVersion(modelID),
		})
	}
	return candidates
}

func claudeLogicalModelID(
	transportModelID string,
	label string,
	models compozyconfig.ProviderModelsConfig,
	candidates []claudeModelCandidate,
) string {
	transportModelID = strings.TrimSpace(transportModelID)
	if transportModelID == "" {
		return ""
	}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.id, transportModelID) {
			return candidate.id
		}
	}
	if strings.EqualFold(transportModelID, "default") {
		return strings.TrimSpace(models.Default)
	}

	family := claudeModelFamily(transportModelID)
	if family == "" {
		family = claudeModelFamily(label)
	}
	matching := make([]claudeModelCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.family != family || family == "" {
			continue
		}
		if claudeLabelIdentifiesCandidate(label, candidate) {
			matching = append(matching, candidate)
		}
	}
	if len(matching) == 0 && family != "" {
		for _, candidate := range candidates {
			if candidate.family == family {
				matching = append(matching, candidate)
			}
		}
	}
	if len(matching) == 0 {
		return transportModelID
	}
	slices.SortFunc(matching, func(left claudeModelCandidate, right claudeModelCandidate) int {
		if versionOrder := compareClaudeVersions(left.version, right.version); versionOrder != 0 {
			return -versionOrder
		}
		return cmp.Compare(left.id, right.id)
	})
	return matching[0].id
}

func claudeLiveDisplayName(
	modelID string,
	label string,
	candidates []claudeModelCandidate,
) string {
	trimmedLabel := strings.TrimSpace(label)
	if trimmedLabel != "" && !strings.EqualFold(trimmedLabel, "default") {
		return trimmedLabel
	}
	for _, candidate := range candidates {
		if candidate.id == modelID && candidate.displayName != "" {
			return candidate.displayName
		}
	}
	return modelID
}

func claudeLabelIdentifiesCandidate(label string, candidate claudeModelCandidate) bool {
	labelTokens := claudeModelTokens(label)
	if len(labelTokens) == 0 {
		return false
	}
	for _, token := range claudeModelTokens(candidate.id) {
		if token == "claude" || token == "1m" {
			continue
		}
		if !slices.Contains(labelTokens, token) {
			return false
		}
	}
	return true
}

func claudeModelFamily(value string) string {
	tokens := claudeModelTokens(value)
	for _, token := range tokens {
		if token == "claude" || token == "default" || token == "1m" || isClaudeNumericToken(token) {
			continue
		}
		return token
	}
	return ""
}

func claudeModelVersion(value string) []int {
	tokens := claudeModelTokens(value)
	familySeen := false
	version := make([]int, 0, 2)
	for _, token := range tokens {
		if token == "claude" || token == "1m" {
			continue
		}
		if !familySeen {
			if isClaudeNumericToken(token) {
				continue
			}
			familySeen = true
			continue
		}
		if number, err := strconv.Atoi(token); err == nil {
			version = append(version, number)
		}
	}
	return version
}

func compareClaudeVersions(left []int, right []int) int {
	for index := range max(len(left), len(right)) {
		leftValue := 0
		if index < len(left) {
			leftValue = left[index]
		}
		rightValue := 0
		if index < len(right) {
			rightValue = right[index]
		}
		if order := cmp.Compare(leftValue, rightValue); order != 0 {
			return order
		}
	}
	return 0
}

func claudeModelTokens(value string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(value)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func isClaudeNumericToken(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if !unicode.IsDigit(char) {
			return false
		}
	}
	return true
}
