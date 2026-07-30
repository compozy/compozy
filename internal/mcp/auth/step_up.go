package auth

import (
	"context"
	"errors"
	"strings"
)

// BeginStepUp starts a new authorization flow with explicitly approved scopes.
func (s *Service) BeginStepUp(
	ctx context.Context,
	cfg ServerConfig,
	redirectURL string,
	approvedScopes []string,
	approved bool,
) (LoginState, error) {
	if !approved {
		return LoginState{}, errors.New("mcp auth: explicit scope approval is required for step-up")
	}
	cfg.Scopes = unionScopes(cfg.Scopes, approvedScopes)
	return s.BeginLogin(ctx, cfg, redirectURL)
}

func unionScopes(current []string, additional []string) []string {
	seen := make(map[string]struct{}, len(current)+len(additional))
	result := make([]string, 0, len(current)+len(additional))
	for _, scope := range append(append([]string(nil), current...), additional...) {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
