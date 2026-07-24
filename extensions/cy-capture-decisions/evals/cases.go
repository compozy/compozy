package evals

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func allCases() []evalCase {
	return []evalCase{
		{"E2E-001", "promotion", runE2E001},
		{"E2E-002", "reconciliation and deviation", runE2E002},
		{"E2E-003", "relevance gate", runE2E003},
		{"E2E-004", "classification and numbering", runE2E004},
		{"E2E-005", "fresh-eyes evidence", runE2E005},
		{"E2E-006", "idempotency", runE2E006},
		{"E2E-007", "terse index", runE2E007},
		{"E2E-008", "rich body", runE2E008},
		{"E2E-009", "candidate lifecycle", runE2E009},
		{"E2E-010", "supersession", runE2E010},
		{"E2E-011", "auto-loaded consumption", runE2E011},
		{"E2E-012", "on-demand body consumption", runE2E012},
		{"E2E-013", "capture as final step", runE2E013},
		{"E2E-014", "before and after review", runE2E014},
		{"E2E-015", "manual correction", runE2E015},
		{"E2E-016", "degraded mode", runE2E016},
		{"E2E-017", "no promotable decisions", runE2E017},
		{"E2E-018", "bad slug", runE2E018},
		{"E2E-019", "malformed source ADR", runE2E019},
		{"E2E-020", "weak semantic match", runE2E020},
		{"E2E-021", "interrupted serial recovery", runE2E021},
		{"IT-001", "capture wiring", runIT001},
		{"IT-005", "unwritable target", runIT005},
		{"IT-006", "VCS reviewability", runIT006},
	}
}

func runE2E001(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 1 {
		return fmt.Errorf("record count = %d, want 1", len(snapshot.Records))
	}
	record, err := requireStatus(snapshot, "feat-orders", "adrs/adr-002.md", "proven")
	if err != nil {
		return err
	}
	return requireIndexed(snapshot, record.ID, true)
}

func runE2E002(ctx context.Context, t *trial) error {
	orders, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, orders, "feat-orders"); err != nil {
		return err
	}
	ordersLog, err := loadLog(orders.Root)
	if err != nil {
		return err
	}
	ordersRecord, err := findByProvenance(ordersLog, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	ordersBody, err := readRecordBody(orders.Root, ordersRecord.ID)
	if err != nil {
		return err
	}
	if err := requireContains(ordersBody, "implemented as designed", "orders body"); err != nil {
		return err
	}
	if err := requireNotContains(ordersBody, "[DEVIATION]", "orders body"); err != nil {
		return err
	}

	payments, err := canonicalWorkspace(ctx, t, "payments", "feat-payments")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, payments, "feat-payments"); err != nil {
		return err
	}
	paymentsLog, err := loadLog(payments.Root)
	if err != nil {
		return err
	}
	paymentRecord, err := findByProvenance(paymentsLog, "feat-payments", "adrs/adr-001.md")
	if err != nil {
		return err
	}
	paymentBody, err := readRecordBody(payments.Root, paymentRecord.ID)
	if err != nil {
		return err
	}
	return requireContains(paymentBody, "[DEVIATION]", "payments body")
}

func runE2E003(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	output, err := t.capture(ctx, w, "feat-orders")
	if err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 1 {
		return fmt.Errorf("record count = %d, want exactly one relevant ADR", len(snapshot.Records))
	}
	for _, needle := range []string{"adr-001", "adr-003", "skip"} {
		if err := requireContains(output, needle, "capture summary"); err != nil {
			return err
		}
	}
	return nil
}

func runE2E004(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	if err := addSecondDurableDecision(ctx, t, w); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 2 {
		return fmt.Errorf("record count = %d, want 2", len(snapshot.Records))
	}
	for _, id := range []string{"AD-001", "AD-002"} {
		if _, ok := snapshot.Records[id]; !ok {
			return fmt.Errorf("missing sequential record %s", id)
		}
	}
	return nil
}

func runE2E005(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "search", "feat-search")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-search"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	proven, err := requireStatus(snapshot, "feat-search", "adrs/adr-001.md", "proven")
	if err != nil {
		return err
	}
	candidate, err := requireStatus(snapshot, "feat-search", "adrs/adr-002.md", "candidate")
	if err != nil {
		return err
	}
	if err := requireIndexed(snapshot, proven.ID, true); err != nil {
		return err
	}
	if err := requireIndexed(snapshot, candidate.ID, false); err != nil {
		return err
	}

	withoutMemory, err := canonicalWorkspace(ctx, t, "search-no-memory", "feat-search")
	if err != nil {
		return err
	}
	memoryDir := filepath.Join(withoutMemory.Root, ".compozy", "tasks", "feat-search", "memory")
	if err := os.RemoveAll(memoryDir); err != nil {
		return err
	}
	if err := commitAll(ctx, t, withoutMemory, "remove memory hint"); err != nil {
		return err
	}
	if _, err := t.capture(ctx, withoutMemory, "feat-search"); err != nil {
		return err
	}
	withoutMemoryLog, err := loadLog(withoutMemory.Root)
	if err != nil {
		return err
	}
	_, err = requireStatus(withoutMemoryLog, "feat-search", "adrs/adr-001.md", "proven")
	return err
}

func runE2E006(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	before, err := treeDigest(filepath.Join(w.Root, ".compozy"))
	if err != nil {
		return err
	}
	output, err := t.capture(ctx, w, "feat-orders")
	if err != nil {
		return err
	}
	after, err := treeDigest(filepath.Join(w.Root, ".compozy"))
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("unchanged rerun modified decision bodies")
	}
	return requireContains(output, "no changes", "idempotent rerun summary")
}

func runE2E007(ctx context.Context, t *trial) error {
	search, err := canonicalWorkspace(ctx, t, "search", "feat-search")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, search, "feat-search"); err != nil {
		return err
	}
	searchLog, err := loadLog(search.Root)
	if err != nil {
		return err
	}
	candidate, err := requireStatus(searchLog, "feat-search", "adrs/adr-002.md", "candidate")
	if err != nil {
		return err
	}
	if err := requireIndexed(searchLog, candidate.ID, false); err != nil {
		return err
	}

	auth, err := seededAuthWorkspace(ctx, t, "auth")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, auth, "feat-auth"); err != nil {
		return err
	}
	authLog, err := loadLog(auth.Root)
	if err != nil {
		return err
	}
	return requireIndexed(authLog, "AD-001", false)
}

func runE2E008(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	record, err := findByProvenance(snapshot, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	body, err := readRecordBody(w.Root, record.ID)
	if err != nil {
		return err
	}
	sections := []string{"## Context", "## Decision", "## Alternatives", "## Consequences", "## Reconciliation"}
	for _, section := range sections {
		if err := requireContains(body, section, "rich decision body"); err != nil {
			return err
		}
	}
	return nil
}

func runE2E009(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "search", "feat-search")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-search"); err != nil {
		return err
	}
	phaseOne, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	candidate, err := requireStatus(phaseOne, "feat-search", "adrs/adr-002.md", "candidate")
	if err != nil {
		return err
	}
	patch := filepath.Join(t.harness.config.ExtensionDir, "evals", "fixtures", "feat-search", "diff-phase2.patch")
	if _, err := t.harness.gitOutput(ctx, w.Root, "apply", patch); err != nil {
		return err
	}
	if err := commitAll(ctx, t, w, "add BM25 evidence"); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-search"); err != nil {
		return err
	}
	phaseTwo, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	proven, err := requireStatus(phaseTwo, "feat-search", "adrs/adr-002.md", "proven")
	if err != nil {
		return err
	}
	if proven.ID != candidate.ID {
		return fmt.Errorf("candidate id %s changed to %s", candidate.ID, proven.ID)
	}
	return requireIndexed(phaseTwo, proven.ID, true)
}

func runE2E010(ctx context.Context, t *trial) error {
	w, err := seededAuthWorkspace(ctx, t, "auth-chain")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-auth"); err != nil {
		return err
	}
	first, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if err := assertSupersession(first, "AD-001", "AD-002"); err != nil {
		return err
	}
	if err := addAuthReversalWorkflow(ctx, t, w); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-auth-v2"); err != nil {
		return err
	}
	chain, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if err := assertSupersession(chain, "AD-002", "AD-003"); err != nil {
		return err
	}
	if len(chain.Indexed) != 1 {
		return fmt.Errorf("active index size = %d, want chain tail only", len(chain.Indexed))
	}
	return requireIndexed(chain, "AD-003", true)
}

func runE2E011(ctx context.Context, t *trial) error {
	w, err := t.newWorkspace(ctx, "consume-index", workspaceOptions{})
	if err != nil {
		return err
	}
	if err := writeConsumptionLog(w.Root); err != nil {
		return err
	}
	if err := writeConsumptionInstructions(w.Root); err != nil {
		return err
	}
	prompt := "State the active project decision about order writes available to this fresh session."
	output, err := t.runModel(ctx, w, prompt)
	if err != nil {
		return err
	}
	if err := requireContains(output, "AD-001", "fresh session output"); err != nil {
		return err
	}
	if err := requireContains(output, "safe retries without duplicate writes", "fresh session output"); err != nil {
		return err
	}
	return requireContains(output, "idempotency", "fresh session output")
}

func runE2E012(ctx context.Context, t *trial) error {
	w, err := t.newWorkspace(ctx, "consume-body", workspaceOptions{})
	if err != nil {
		return err
	}
	if err := writeConsumptionLog(w.Root); err != nil {
		return err
	}
	if err := writeConsumptionInstructions(w.Root); err != nil {
		return err
	}
	prompt := "A new orders feature needs safe retry behavior. Use the loaded project decision index, " +
		"read the relevant rich decision body on demand, and explain the governing constraint. " +
		"Do not read unrelated decision bodies."
	output, err := t.runModel(ctx, w, prompt)
	if err != nil {
		return err
	}
	if err := requireContains(output, "Clients retry writes after transport failures", "raw ACP events"); err != nil {
		return err
	}
	if err := requireNotContains(output, "Historical invoices must remain reproducible", "raw ACP events"); err != nil {
		return err
	}
	successorPrompt := "A teammate cited retired decision AD-002. " +
		"Read that rich body and identify the active successor."
	successorOutput, err := t.runModel(ctx, w, successorPrompt)
	if err != nil {
		return err
	}
	retiredContext := "Early clients retried requests without stable identifiers"
	if err := requireContains(successorOutput, retiredContext, "superseded body raw ACP events"); err != nil {
		return err
	}
	return requireContains(successorOutput, "AD-001", "superseded decision successor")
}

func runE2E013(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-reviewed", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	record, err := findByProvenance(snapshot, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	if err := requireContains(record.Evidence, "issue_001", "review-backed evidence"); err != nil {
		return err
	}
	return requireContains(record.Evidence, "completed", "final-step evidence")
}

func runE2E014(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-review-timing", "feat-orders")
	if err != nil {
		return err
	}
	reviewDir := filepath.Join(w.Root, ".compozy", "tasks", "feat-orders", "reviews-001")
	backup := filepath.Join(t.harness.runtimeDir, fmt.Sprintf("review-backup-%s-%d", t.caseID, t.number))
	if err := copyTree(reviewDir, backup); err != nil {
		return err
	}
	if err := os.RemoveAll(reviewDir); err != nil {
		return err
	}
	if err := commitAll(ctx, t, w, "remove review evidence"); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	before, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	beforeRecord, err := findByProvenance(before, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	if err := copyTree(backup, reviewDir); err != nil {
		return err
	}
	if err := commitAll(ctx, t, w, "restore review evidence"); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	after, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	afterRecord, err := findByProvenance(after, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	if beforeRecord.ID != afterRecord.ID {
		return fmt.Errorf("review rerun changed id from %s to %s", beforeRecord.ID, afterRecord.ID)
	}
	return requireContains(afterRecord.Evidence, "issue_001", "updated evidence")
}

func runE2E015(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-manual", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	record, err := findByProvenance(snapshot, "feat-orders", "adrs/adr-002.md")
	if err != nil {
		return err
	}
	path := filepath.Join(w.Root, ".compozy", "decisions", record.ID+".md")
	const correction = "Manual correction: publication is synchronous and ordered."
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if _, err := file.WriteString("\n" + correction + "\n"); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	body, err := readRecordBody(w.Root, record.ID)
	if err != nil {
		return err
	}
	if err := requireContains(body, correction, "manually corrected record"); err != nil {
		return err
	}
	after, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(after.Records) != 1 {
		return fmt.Errorf("manual correction rerun created %d records, want 1", len(after.Records))
	}
	return nil
}

func runE2E016(ctx context.Context, t *trial) error {
	degraded, err := t.newWorkspace(ctx, "telemetry-degraded", workspaceOptions{
		Fixture:      "feat-telemetry",
		NoMainBranch: true,
	})
	if err != nil {
		return err
	}
	output, err := t.capture(ctx, degraded, "feat-telemetry")
	if err != nil {
		return err
	}
	snapshot, err := loadLog(degraded.Root)
	if err != nil {
		return err
	}
	candidate, err := requireStatus(snapshot, "feat-telemetry", "adrs/adr-001.md", "candidate")
	if err != nil {
		return err
	}
	if err := requireIndexed(snapshot, candidate.ID, false); err != nil {
		return err
	}
	if err := requireContains(output, "degraded", "degraded run summary"); err != nil {
		return err
	}

	noEvidence, err := t.newWorkspace(ctx, "telemetry-no-evidence", workspaceOptions{
		Fixture:      "feat-telemetry",
		NoMainBranch: true,
	})
	if err != nil {
		return err
	}
	reviewsDir := filepath.Join(noEvidence.Root, ".compozy", "tasks", "feat-telemetry", "reviews-001")
	if err := os.RemoveAll(reviewsDir); err != nil {
		return err
	}
	if err := commitAll(ctx, t, noEvidence, "remove review evidence"); err != nil {
		return err
	}
	if _, err := t.capture(ctx, noEvidence, "feat-telemetry"); err != nil {
		return err
	}
	return requireNoDecisionBodies(noEvidence.Root)
}

func runE2E017(ctx context.Context, t *trial) error {
	existing, err := t.newWorkspace(ctx, "noop-existing", workspaceOptions{Fixture: "feat-noop"})
	if err != nil {
		return err
	}
	if err := writeConsumptionLog(existing.Root); err != nil {
		return err
	}
	before, err := treeDigest(filepath.Join(existing.Root, ".compozy"))
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, existing, "feat-noop"); err != nil {
		return err
	}
	after, err := treeDigest(filepath.Join(existing.Root, ".compozy"))
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("no-op capture changed existing decision log")
	}

	fresh, err := t.newWorkspace(ctx, "noop-fresh", workspaceOptions{Fixture: "feat-noop"})
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, fresh, "feat-noop"); err != nil {
		return err
	}
	hasLog, err := hasDecisionLog(fresh.Root)
	if err != nil {
		return err
	}
	if hasLog {
		return errors.New("fresh no-op capture created an unnecessary index")
	}
	return requireNoDecisionBodies(fresh.Root)
}

func runE2E018(ctx context.Context, t *trial) error {
	for _, scenario := range []struct {
		name   string
		prompt string
	}{
		{name: "missing-slug", prompt: "Use the cy-capture-decisions skill with no workflow slug."},
		{name: "unknown-slug", prompt: "Use the cy-capture-decisions skill to capture no-such-slug."},
	} {
		w, err := t.newWorkspace(ctx, scenario.name, workspaceOptions{})
		if err != nil {
			return err
		}
		output, runErr := t.runModel(ctx, w, scenario.prompt)
		if runErr != nil {
			return runErr
		}
		if err := requireContains(output, "slug", "bad-slug response"); err != nil {
			return err
		}
		hasLog, statErr := hasDecisionLog(w.Root)
		if statErr != nil {
			return statErr
		}
		if hasLog {
			return fmt.Errorf("%s scenario wrote a decision log", scenario.name)
		}
	}

	archived, err := t.newWorkspace(ctx, "archived-slug", workspaceOptions{})
	if err != nil {
		return err
	}
	source := filepath.Join(t.harness.config.ExtensionDir, "evals", "fixtures", "feat-orders", "workflow")
	target := filepath.Join(archived.Root, ".compozy", "tasks", "_archived", "2026-07-15-feat-orders")
	if err := copyTree(source, target); err != nil {
		return err
	}
	if err := commitAll(ctx, t, archived, "archive fixture"); err != nil {
		return err
	}
	output, err := t.capture(ctx, archived, "feat-orders")
	if err != nil {
		return err
	}
	if err := requireContains(output, "archived", "archived-slug response"); err != nil {
		return err
	}
	hasLog, err := hasDecisionLog(archived.Root)
	if err != nil {
		return err
	}
	if hasLog {
		return errors.New("archived slug wrote a decision log")
	}
	return nil
}

func runE2E019(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "payments", "feat-payments")
	if err != nil {
		return err
	}
	output, err := t.capture(ctx, w, "feat-payments")
	if err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 1 {
		return fmt.Errorf("malformed ADR run produced %d records, want 1", len(snapshot.Records))
	}
	if _, err := requireStatus(snapshot, "feat-payments", "adrs/adr-001.md", "proven"); err != nil {
		return err
	}
	return requireContains(output, "adr-002", "malformed ADR warning")
}

func runE2E020(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "search-weak", "feat-search")
	if err != nil {
		return err
	}
	if err := writeWeakMatchSeed(w.Root); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-search"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 2 {
		return fmt.Errorf("weak-match capture has %d records, want exactly prior plus NEW", len(snapshot.Records))
	}
	if _, ok := snapshot.Records["AD-001"]; !ok {
		return errors.New("weak-match capture removed the prior AD-001")
	}
	newRecord, err := findByProvenance(snapshot, "feat-search", "adrs/adr-001.md")
	if err != nil {
		return err
	}
	if newRecord.ID == "AD-001" {
		return errors.New("weak semantic match incorrectly updated AD-001")
	}
	return nil
}

func runE2E021(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-partial", "feat-orders")
	if err != nil {
		return err
	}
	decisionDir := filepath.Join(w.Root, ".compozy", "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		return err
	}
	partial := filepath.Join(decisionDir, "AD-001.md")
	if err := os.WriteFile(partial, []byte("---\nid: AD-001\nstatus:"), 0o600); err != nil {
		return err
	}
	indexPath := filepath.Join(w.Root, ".compozy", "DECISIONS.md")
	if err := os.WriteFile(indexPath, []byte("# Project Decisions (active, proven)\n"), 0o600); err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	snapshot, err := loadLog(w.Root)
	if err != nil {
		return err
	}
	if len(snapshot.Records) != 1 {
		return fmt.Errorf("serial recovery produced %d records, want 1", len(snapshot.Records))
	}
	_, err = requireStatus(snapshot, "feat-orders", "adrs/adr-002.md", "proven")
	return err
}

func runIT001(ctx context.Context, t *trial) error {
	return runE2E001(ctx, t)
}

func runIT005(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-unwritable", "feat-orders")
	if err != nil {
		return err
	}
	if err := writeConsumptionLog(w.Root); err != nil {
		return err
	}
	logRoot := filepath.Join(w.Root, ".compozy")
	before, err := treeDigest(logRoot)
	if err != nil {
		return err
	}
	if err := setTreePermissions(logRoot, 0o555, 0o444); err != nil {
		return err
	}
	writable, probeErr := treeAllowsCreate(logRoot)
	if probeErr != nil {
		restoreErr := setTreePermissions(logRoot, 0o755, 0o644)
		return errors.Join(probeErr, restoreErr)
	}
	if writable {
		if err := setTreePermissions(logRoot, 0o755, 0o644); err != nil {
			return err
		}
		return skipEval("filesystem does not enforce chmod write restrictions for this process")
	}
	output, runErr := t.capture(ctx, w, "feat-orders")
	restoreErr := setTreePermissions(logRoot, 0o755, 0o644)
	if restoreErr != nil {
		return restoreErr
	}
	after, err := treeDigest(logRoot)
	if err != nil {
		return err
	}
	if before != after {
		return errors.New("unwritable target changed despite write failure contract")
	}
	failureEvidence := output
	if runErr != nil {
		failureEvidence += "\n" + runErr.Error()
	}
	for _, needle := range []string{"permission denied", "unwritable", "read-only", "failed", "cannot write"} {
		if strings.Contains(strings.ToLower(failureEvidence), needle) {
			return nil
		}
	}
	return errors.New("unwritable run preserved the log but did not report a write failure")
}

func runIT006(ctx context.Context, t *trial) error {
	w, err := canonicalWorkspace(ctx, t, "orders-vcs", "feat-orders")
	if err != nil {
		return err
	}
	if _, err := t.capture(ctx, w, "feat-orders"); err != nil {
		return err
	}
	status, err := t.harness.gitOutput(ctx, w.Root, "status", "--short", "--", ".compozy")
	if err != nil {
		return err
	}
	if err := requireContains(status, "?? .compozy/", "decision-log git status"); err != nil {
		return err
	}
	if _, err := t.harness.gitOutput(ctx, w.Root, "add", "--intent-to-add", ".compozy"); err != nil {
		return err
	}
	diff, err := t.harness.gitOutput(ctx, w.Root, "diff", "--", ".compozy")
	if err != nil {
		return err
	}
	if strings.TrimSpace(diff) == "" {
		return errors.New("captured decision log is not a reviewable Git diff")
	}
	for _, needle := range []string{"DECISIONS.md", "AD-001.md"} {
		if err := requireContains(diff, needle, "decision-log git diff"); err != nil {
			return err
		}
	}
	return nil
}

func canonicalWorkspace(ctx context.Context, t *trial, name, fixture string) (*workspace, error) {
	return t.newWorkspace(ctx, name, workspaceOptions{Fixture: fixture})
}

func seededAuthWorkspace(ctx context.Context, t *trial, name string) (*workspace, error) {
	return t.newWorkspace(ctx, name, workspaceOptions{Fixture: "feat-auth", SeedLog: true})
}

func commitAll(ctx context.Context, t *trial, w *workspace, message string) error {
	if _, err := t.harness.gitOutput(ctx, w.Root, "add", "."); err != nil {
		return err
	}
	_, err := t.harness.gitOutput(ctx, w.Root, "commit", "-q", "--allow-empty", "-m", message)
	return err
}

func readRecordBody(workspaceRoot, id string) (string, error) {
	content, err := os.ReadFile(filepath.Join(workspaceRoot, ".compozy", "decisions", id+".md"))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func addSecondDurableDecision(ctx context.Context, t *trial, w *workspace) error {
	adr := `# ADR-004: Causation ids on every domain event

- Status: Accepted
- Date: 2026-07-11

## Context

Every future asynchronous feature must trace why an order event exists.

## Decision

Attach a causation id to every published domain event and preserve it across handlers.

## Alternatives Considered

- Per-service trace ids (rejected: breaks cross-feature audit continuity).

## Consequences

- Retries and audit tooling share one durable correlation contract.
`
	code := `package orders

type Envelope struct {
	Event       Event
	CausationID string
}
`
	base := filepath.Join(w.Root, ".compozy", "tasks", "feat-orders", "adrs")
	if err := os.WriteFile(filepath.Join(base, "adr-004.md"), []byte(adr), 0o600); err != nil {
		return err
	}
	codePath := filepath.Join(w.Root, "internal", "orders", "envelope.go")
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(codePath, []byte(code), 0o600); err != nil {
		return err
	}
	return commitAll(ctx, t, w, "add second durable decision")
}

func addAuthReversalWorkflow(ctx context.Context, t *trial, w *workspace) error {
	root := filepath.Join(w.Root, ".compozy", "tasks", "feat-auth-v2")
	if err := os.MkdirAll(filepath.Join(root, "adrs"), 0o755); err != nil {
		return err
	}
	adr := `# ADR-001: Proof-of-possession tokens replace server sessions

- Status: Accepted
- Date: 2026-07-15

## Context

Offline clients cannot depend on the server-side session store chosen by feat-auth.

## Decision

Replace server-side sessions with short-lived proof-of-possession tokens and a revocation epoch.

## Alternatives Considered

- Keep server sessions (rejected: offline verification is impossible).

## Consequences

- Every future auth consumer must validate the proof key and revocation epoch.
`
	task := "---\nstatus: completed\ntitle: Replace sessions\ntype: feature\n---\n\n" +
		"Verified token validation and revocation epoch behavior.\n"
	if err := os.WriteFile(filepath.Join(root, "adrs", "adr-001.md"), []byte(adr), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "task_01.md"), []byte(task), 0o600); err != nil {
		return err
	}
	codePath := filepath.Join(w.Root, "internal", "auth", "proof_tokens.go")
	code := "package auth\n\n// ProofToken replaces server-side session state.\n" +
		"type ProofToken struct { RevocationEpoch int64 }\n"
	if err := os.MkdirAll(filepath.Dir(codePath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(codePath, []byte(code), 0o600); err != nil {
		return err
	}
	return commitAll(ctx, t, w, "reverse session decision")
}

func assertSupersession(snapshot logSnapshot, oldID, newID string) error {
	oldRecord, ok := snapshot.Records[oldID]
	if !ok {
		return fmt.Errorf("missing superseded record %s", oldID)
	}
	newRecord, ok := snapshot.Records[newID]
	if !ok {
		return fmt.Errorf("missing successor record %s", newID)
	}
	if oldRecord.Status != "superseded" || oldRecord.SupersededBy != newID {
		return fmt.Errorf("%s supersession metadata is inconsistent", oldID)
	}
	found := false
	for _, id := range newRecord.Supersedes {
		if id == oldID {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("%s does not supersede %s", newID, oldID)
	}
	if err := requireIndexed(snapshot, oldID, false); err != nil {
		return err
	}
	return requireIndexed(snapshot, newID, true)
}

func writeConsumptionLog(root string) error {
	decisionDir := filepath.Join(root, ".compozy", "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		return err
	}
	if err := writeConsumptionIndex(root); err != nil {
		return err
	}
	for id, write := range map[string]func(string) error{
		"AD-001.md": writeActiveConsumptionBody,
		"AD-002.md": writeRetiredConsumptionBody,
		"AD-003.md": writeUnrelatedConsumptionBody,
	} {
		if err := write(filepath.Join(decisionDir, id)); err != nil {
			return err
		}
	}
	return nil
}

func writeConsumptionInstructions(root string) error {
	content := `# Project decision memory

Before planning or implementation, read .compozy/DECISIONS.md once in the fresh session.
Read a matching .compozy/decisions/AD-NNN.md body only when the current work needs its details.

@.compozy/DECISIONS.md
`
	return os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(content), 0o600)
}

func writeConsumptionIndex(root string) error {
	index := "# Project Decisions (active, proven)\n\n" +
		"# Imported into agent memory. One line per active, proven decision.\n" +
		"# Rich bodies: .compozy/decisions/AD-NNN.md\n\n" +
		"AD-001 | Idempotency keys on order writes | proven | [orders, api] | " +
		"safe retries without duplicate writes | feat-orders\n" +
		"AD-003 | Regional tax tables are versioned | proven | [billing, tax] | " +
		"audits reproduce historical invoices | feat-billing\n"
	path := filepath.Join(root, ".compozy", "DECISIONS.md")
	return os.WriteFile(path, []byte(index), 0o600)
}

func writeActiveConsumptionBody(path string) error {
	body := `---
id: AD-001
title: Idempotency keys on order writes
status: proven
tags: [orders, api]
source_slug: feat-orders
source_adr: adrs/adr-002.md
promoted_at: 2026-07-15
supersedes: [AD-002]
superseded_by: null
evidence: "integration test proves duplicate command ids produce one event"
---

## Context

Clients retry writes after transport failures.

## Decision

Every order write carries a stable idempotency key.

## Alternatives

- At-least-once duplicate writes (rejected).

## Consequences

- Retries are safe.

## Reconciliation

Implemented as designed and proven by integration tests.
`
	return os.WriteFile(path, []byte(body), 0o600)
}

func writeRetiredConsumptionBody(path string) error {
	body := `---
id: AD-002
title: Retry order writes without stable keys
status: superseded
tags: [orders, api]
source_slug: feat-orders-v1
source_adr: adrs/adr-001.md
promoted_at: 2026-07-10
supersedes: []
superseded_by: AD-001
evidence: "superseded after duplicate-write incident review"
---

## Context

Early clients retried requests without stable identifiers.

## Decision

Permit transport retries without an idempotency key.

## Alternatives

- Stable idempotency keys.

## Consequences

- Duplicate writes are possible.

## Reconciliation

Retired by AD-001 after production evidence showed duplicate writes.
`
	return os.WriteFile(path, []byte(body), 0o600)
}

func writeUnrelatedConsumptionBody(path string) error {
	body := `---
id: AD-003
title: Regional tax tables are versioned
status: proven
tags: [billing, tax]
source_slug: feat-billing
source_adr: adrs/adr-004.md
promoted_at: 2026-07-12
supersedes: []
superseded_by: null
evidence: "invoice replay integration tests"
---

## Context

Historical invoices must remain reproducible.

## Decision

Version regional tax tables.

## Alternatives

- Use only current rates.

## Consequences

- Invoice records retain the applied table version.

## Reconciliation

Implemented as designed.
`
	return os.WriteFile(path, []byte(body), 0o600)
}

func writeWeakMatchSeed(root string) error {
	decisionDir := filepath.Join(root, ".compozy", "decisions")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		return err
	}
	index := "# Project Decisions (active, proven)\n\n" +
		"AD-001 | Search index strategy | proven | [catalog] | " +
		"catalog lookup convention | feat-catalog\n"
	body := `---
id: AD-001
title: Search index strategy
status: proven
tags: [catalog]
source_slug: feat-catalog
source_adr: adrs/adr-009.md
promoted_at: 2026-07-10
supersedes: []
superseded_by: null
evidence: "catalog integration tests"
---

## Context

Catalog lookups need an index.

## Decision

Use a sorted catalog index.

## Alternatives

- Scan all rows.

## Consequences

- Catalog lookups are bounded.

## Reconciliation

Implemented as designed.
`
	if err := os.WriteFile(filepath.Join(root, ".compozy", "DECISIONS.md"), []byte(index), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(decisionDir, "AD-001.md"), []byte(body), 0o600)
}

func requireNoDecisionBodies(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, ".compozy", "decisions"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "AD-") && strings.HasSuffix(entry.Name(), ".md") {
			return fmt.Errorf("unexpected decision body %s", entry.Name())
		}
	}
	return nil
}

func hasDecisionLog(root string) (bool, error) {
	_, err := os.Stat(filepath.Join(root, ".compozy", "DECISIONS.md"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func setTreePermissions(root string, dirMode, fileMode fs.FileMode) error {
	var paths []string
	err := filepath.WalkDir(root, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		mode := fileMode
		if info.IsDir() {
			mode = dirMode
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
	}
	return nil
}

func treeAllowsCreate(root string) (bool, error) {
	probe := filepath.Join(root, ".write-probe")
	file, err := os.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, nil
	}
	return true, errors.Join(file.Close(), os.Remove(probe))
}
