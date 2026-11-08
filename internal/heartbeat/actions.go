package heartbeat

import (
	"encoding/json/v2"

	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ActionDecl struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	AutoAccept   bool   `json:"auto_accept"`
}

var actionIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

func parseActions(value any) ([]ActionDecl, error) {
	if _, ok := value.([]any); !ok {
		return nil, fmt.Errorf("heartbeat_action_invalid: actions must be a list")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var actions []ActionDecl

	if err := json.Unmarshal(data, &actions, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("heartbeat_action_invalid: %w", err)
	}
	if len(actions) > 8 {
		return nil, fmt.Errorf("heartbeat_action_limit_exceeded: at most 8 actions")
	}
	seen := map[string]bool{}
	for _, a := range actions {
		if !actionIDPattern.MatchString(a.ID) || strings.TrimSpace(a.Title) == "" ||
			utf8.RuneCountInString(a.Title) > 120 ||
			strings.TrimSpace(a.Instructions) == "" ||
			utf8.RuneCountInString(a.Instructions) > 1000 {
			return nil, fmt.Errorf("heartbeat_action_invalid: invalid id, title or instructions")
		}
		if seen[a.ID] {
			return nil, fmt.Errorf("heartbeat_action_duplicate_id: duplicate action id")
		}
		seen[a.ID] = true
	}
	return actions, nil
}

func AuthorityActions(actions []ActionDecl) []AuthorityAction {
	out := make([]AuthorityAction, 0, len(actions))
	for _, a := range actions {
		out = append(out, AuthorityAction{ID: a.ID, Instructions: a.Instructions, AutoAccept: a.AutoAccept})
	}
	return out
}
