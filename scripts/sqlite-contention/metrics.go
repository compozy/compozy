package main

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"modernc.org/sqlite"
)

type measurements struct {
	core      string
	active    atomic.Bool
	commits   [2]atomic.Int64
	rollbacks [2]atomic.Int64
	mu        sync.Mutex
	samples   map[string][]float64
	failures  map[string][]string
}

func newMeasurements(core string) *measurements {
	return &measurements{core: core, samples: make(map[string][]float64), failures: make(map[string][]string)}
}
func (m *measurements) install() error {
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
		raw, _, _ := strings.Cut(dsn, "?")
		path, err := url.PathUnescape(strings.TrimPrefix(raw, "file:"))
		if err != nil {
			return err
		}
		path, err = filepath.Abs(path)
		if err != nil {
			return err
		}
		index := -1
		switch path {
		case m.core:
			index = 0
		case m.core + ".telemetry.db":
			index = 1
		}
		if index < 0 {
			return nil
		}
		hooks, ok := conn.(sqlite.HookRegisterer)
		if !ok {
			return fmt.Errorf("driver lacks commit hooks")
		}
		hooks.RegisterCommitHook(func() int32 {
			if m.active.Load() {
				m.commits[index].Add(1)
			}
			return 0
		})
		hooks.RegisterRollbackHook(func() {
			if m.active.Load() {
				m.rollbacks[index].Add(1)
			}
		})
		return nil
	})
	return nil
}
func (m *measurements) call(name string, fn func() error) {
	active := m.active.Load()
	start := time.Now()
	err := fn()
	elapsed := float64(time.Since(start)) / float64(time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	if active {
		m.samples[name] = append(m.samples[name], elapsed)
	}
	if err != nil {
		m.failures[name] = append(m.failures[name], err.Error())
	}
}
func (m *measurements) result(start, end time.Time, openMS float64) map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()
	summary := make(map[string]any, len(m.samples))
	for name, samples := range m.samples {
		slices.Sort(samples)
		at := func(p float64) float64 { return samples[min(len(samples)-1, int(float64(len(samples)-1)*p))] }
		summary[name] = map[string]any{
			"count":  len(samples),
			"p50_ms": at(.5),
			"p95_ms": at(.95),
			"p99_ms": at(.99),
			"max_ms": samples[len(samples)-1],
		}
	}
	hooks := map[string]int64{}
	rollbacks := map[string]int64{}
	for i, name := range []string{"core", "telemetry"} {
		hooks[name] = m.commits[i].Load()
		rollbacks[name] = m.rollbacks[i].Load()
	}
	return map[string]any{
		openMetric:           openMS,
		"start":              start,
		"end":                end,
		"seconds":            end.Sub(start).Seconds(),
		"latency":            summary,
		"failures":           m.failures,
		"write_commit_hooks": hooks,
		"rollback_hooks":     rollbacks,
	}
}
