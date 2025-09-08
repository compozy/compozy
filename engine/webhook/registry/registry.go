package registry

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/webhook"
)

type Entry struct {
	WorkflowID string
	Webhook    *webhook.Config
}

type Registry struct {
	mu     sync.RWMutex
	bySlug map[string]Entry
}

var ErrDuplicateSlug = errors.New("duplicate webhook slug")

func New() *Registry { return &Registry{bySlug: map[string]Entry{}} }

type Lookup interface{ Get(string) (Entry, bool) }

func normalizeSlug(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func (r *Registry) Add(slug string, e Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := normalizeSlug(slug)
	if key == "" {
		return fmt.Errorf("slug must not be empty")
	}
	if e.Webhook != nil {
		ps := normalizeSlug(e.Webhook.Slug)
		if ps == "" {
			return fmt.Errorf("entry webhook slug must not be empty")
		}
		if ps != key {
			return fmt.Errorf("slug mismatch: key=%s payload=%s", key, ps)
		}
	}
	if _, ok := r.bySlug[key]; ok {
		return fmt.Errorf("%w: %s", ErrDuplicateSlug, key)
	}
	r.bySlug[key] = e
	return nil
}

func (r *Registry) Get(slug string) (Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.bySlug[normalizeSlug(slug)]
	return e, ok
}

func (r *Registry) Remove(slug string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bySlug, normalizeSlug(slug))
}

func (r *Registry) Slugs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.bySlug))
	for s := range r.bySlug {
		out = append(out, s)
	}
	return out
}
