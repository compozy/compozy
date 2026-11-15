package githubauth

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const lookupTimeout = 5 * time.Second

// Resolver keeps GitHub CLI credentials in memory within its owner's lifetime and environment.
type Resolver struct {
	getenv  func(string) string
	run     func(context.Context) ([]byte, error)
	now     func() time.Time
	mu      sync.Mutex
	token   string
	expires time.Time
}

func NewResolver(getenv func(string) string) *Resolver {
	if getenv == nil {
		getenv = os.Getenv
	}
	return &Resolver{getenv: getenv, now: time.Now, run: func(ctx context.Context) ([]byte, error) {
		command := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com")
		configureCredentialCommand(command)
		command.WaitDelay = time.Second
		command.Env = append(command.Environ(), "GH_PROMPT_DISABLED=1")
		return command.Output()
	}}
}

func (r *Resolver) Resolve(ctx context.Context, origin string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if token := strings.TrimSpace(r.getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}
	// The CLI login belongs to github.com; a configured mirror does not own that credential.
	if !IsAPIOriginURL(origin, "https://api.github.com") {
		return "", nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.now().Before(r.expires) {
		return r.token, nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	raw, err := r.run(lookupCtx)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", ctxErr
	}
	r.token = ""
	if err == nil {
		r.token = strings.TrimSpace(string(raw))
	}
	r.expires = r.now().Add(time.Minute)
	return r.token, nil
}
