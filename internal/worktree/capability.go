package worktree

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/diagnostics"
	"golang.org/x/sys/execabs"
)

type CapabilityDiagnostic struct {
	Available bool
	Code      string
	Message   string
}

type gitVersion struct {
	major int
	minor int
	patch int
}

var minimumGitVersion = gitVersion{major: 2, minor: 37}

type CapabilityGate struct {
	runner   GitRunner
	lookPath func(string) (string, error)
	once     sync.Once
	result   CapabilityDiagnostic
	err      error
}

func NewCapabilityGate(runner GitRunner) *CapabilityGate {
	return &CapabilityGate{runner: runner, lookPath: execabs.LookPath}
}

func NewCapabilityGateWithLookPath(runner GitRunner, lookPath func(string) (string, error)) *CapabilityGate {
	return &CapabilityGate{runner: runner, lookPath: lookPath}
}

func (g *CapabilityGate) Check(ctx context.Context) (CapabilityDiagnostic, error) {
	if g == nil {
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error()}, ErrGitUnavailable
	}
	g.once.Do(func() { g.result, g.err = g.probe(ctx) })
	return g.result, g.err
}

func (g *CapabilityGate) probe(ctx context.Context) (CapabilityDiagnostic, error) {
	if g.lookPath == nil {
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error()}, ErrGitUnavailable
	}
	if _, err := g.lookPath("git"); err != nil {
		detail := diagnostics.RedactAndBound(err.Error(), 2048)
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error(), Message: detail},
			fmt.Errorf("%w: %s", ErrGitUnavailable, detail)
	}
	if g.runner == nil {
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error()}, ErrGitUnavailable
	}
	stdout, stderr, err := g.runner.Run(ctx, "", "--version")
	if err != nil {
		detail := diagnostics.RedactAndBound(strings.TrimSpace(string(stderr)), 2048)
		cause := diagnostics.RedactAndBound(err.Error(), 2048)
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error(), Message: detail},
			fmt.Errorf("%w: probe git version: %s", ErrGitUnavailable, cause)
	}
	version, err := parseGitVersion(string(stdout))
	if err != nil || version.less(minimumGitVersion) {
		found := diagnostics.RedactAndBound(strings.TrimSpace(string(stdout)), 2048)
		return CapabilityDiagnostic{Code: ErrGitVersionUnsupported.Error(), Message: found},
			refusal(ErrGitVersionUnsupported, "found "+found)
	}
	return CapabilityDiagnostic{Available: true}, nil
}

func parseGitVersion(output string) (gitVersion, error) {
	value := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(output), "git version "))
	if value == "" {
		return gitVersion{}, fmt.Errorf("parse empty git version")
	}
	parts := strings.Split(strings.Fields(value)[0], ".")
	if len(parts) < 2 {
		return gitVersion{}, fmt.Errorf("parse git version %q", value)
	}
	values := [3]int{}
	for index := range min(3, len(parts)) {
		digits := leadingVersionDigits(parts[index])
		if digits == "" {
			return gitVersion{}, fmt.Errorf("parse git version %q", value)
		}
		parsed, err := strconv.Atoi(digits)
		if err != nil {
			return gitVersion{}, fmt.Errorf("parse git version %q: %w", value, err)
		}
		values[index] = parsed
	}
	return gitVersion{major: values[0], minor: values[1], patch: values[2]}, nil
}

func leadingVersionDigits(value string) string {
	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return value[:index]
}

func (v gitVersion) less(other gitVersion) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}
