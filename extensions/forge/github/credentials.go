package github

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	credentialLookupTimeout = 5 * time.Second
	credentialSourceBinding = "binding"
)

type credential struct {
	token  string
	source string
}

type credentialResolver struct {
	getenv   func(string) string
	lookPath func(string) (string, error)
	runGH    func(context.Context, string) ([]byte, error)
}

func newCredentialResolver() credentialResolver {
	return credentialResolver{
		getenv:   os.Getenv,
		lookPath: exec.LookPath,
		runGH: func(ctx context.Context, path string) ([]byte, error) {
			return exec.CommandContext(ctx, path, "auth", "token").Output()
		},
	}
}

func (r credentialResolver) resolve(ctx context.Context) (credential, error) {
	if token := strings.TrimSpace(r.getenv("GITHUB_TOKEN")); token != "" {
		return credential{token: token, source: credentialSourceBinding}, nil
	}
	path, err := r.lookPath("gh")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return credential{}, nil
		}
		return credential{}, err
	}
	lookupCtx, cancel := context.WithTimeout(ctx, credentialLookupTimeout)
	defer cancel()
	token, err := r.runGH(lookupCtx, path)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return credential{}, ctxErr
		}
		if lookupErr := lookupCtx.Err(); lookupErr != nil {
			return credential{}, lookupErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return credential{}, err
		}
		return credential{}, nil
	}
	value := strings.TrimSpace(string(token))
	if value == "" {
		return credential{}, nil
	}
	return credential{token: value, source: "gh"}, nil
}
