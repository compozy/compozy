// Command session-pull-request-seeder gives an existing workspace-root session a
// settled branch snapshot and caches the pull request for that branch in a
// stopped isolated daemon home, for the topbar PR chip journeys (E2E-001,
// E2E-003). The facts are observed "now", so the daemon serves them fresh and
// never schedules a forge refresh during the walk.
package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/worktree"
)

type options struct {
	homeDir, workspaceID, sessionID              string
	branch, repoKey, checks, review, failedNames string
	number                                       int
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) (err error) {
	parsed, err := parseOptions(args)
	if err != nil {
		return err
	}
	home, err := compozyconfig.ResolveHomePathsFrom(parsed.homeDir)
	if err != nil {
		return fmt.Errorf("resolve isolated home: %w", err)
	}
	database, err := globaldb.OpenGlobalDB(ctx, home.DatabaseFile)
	if err != nil {
		return fmt.Errorf("open isolated global database (stop the isolated daemon first): %w", err)
	}
	defer func() { err = errors.Join(err, database.Close(ctx)) }()

	if err := seed(ctx, database, parsed); err != nil {
		return err
	}
	encoded, err := json.Marshal(map[string]string{"session_id": parsed.sessionID})
	if err != nil {
		return fmt.Errorf("encode seeded id: %w", err)
	}
	if _, err := os.Stdout.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("print seeded id: %w", err)
	}
	return nil
}

func seed(ctx context.Context, database *globaldb.GlobalDB, parsed options) error {
	now := time.Now().UTC()
	if err := database.SaveSessionBranch(ctx, parsed.workspaceID, parsed.sessionID, parsed.branch); err != nil {
		return fmt.Errorf("save seeded branch: %w", err)
	}
	return database.Worktrees.SavePullRequest(ctx, pullRequestEntry(parsed, now))
}

func pullRequestEntry(parsed options, now time.Time) worktree.PullRequestEntry {
	state, draft, merged := "open", false, false
	number := parsed.number
	checks := &worktree.ForgeChecks{State: parsed.checks, FailedNames: []string{}}
	switch parsed.checks {
	case "passing":
		checks.Passed = 14
	case "failing":
		checks.Passed, checks.Failed = 12, 2
		checks.FailedNames = strings.Split(parsed.failedNames, ",")
	}
	return worktree.PullRequestEntry{
		Key: worktree.PullRequestKey{
			WorkspaceID: parsed.workspaceID,
			RepoKey:     parsed.repoKey,
			Branch:      parsed.branch,
		},
		Facts: &worktree.ForgeStatus{
			Provider:       "github",
			PRNumber:       &number,
			PRState:        &state,
			PRURL:          fmt.Sprintf("https://%s/pull/%d", parsed.repoKey, number),
			Head:           parsed.branch,
			Base:           "main",
			HeadSHA:        "8f3c2a1d40",
			Draft:          &draft,
			Merged:         &merged,
			Checks:         checks,
			ReviewDecision: parsed.review,
			Mergeable:      "mergeable",
			UpdatedAt:      &now,
			FetchedAt:      &now,
			Selection:      "complete",
		},
		LookupState: "ok",
		AttemptAt:   &now,
	}
}

func parseOptions(args []string) (options, error) {
	var parsed options
	flags := flag.NewFlagSet("session-pull-request-seeder", flag.ContinueOnError)
	flags.StringVar(&parsed.homeDir, "home", "", "isolated COMPOZY_HOME")
	flags.StringVar(&parsed.workspaceID, "workspace", "", "workspace id owning the session")
	flags.StringVar(&parsed.sessionID, "session", "", "existing session id")
	flags.StringVar(&parsed.branch, "branch", "", "settled branch snapshot")
	flags.StringVar(&parsed.repoKey, "repo-key", "", "normalized origin host/owner/name")
	flags.IntVar(&parsed.number, "number", 0, "pull request number")
	flags.StringVar(&parsed.checks, "checks", "passing", "checks state: passing or failing")
	flags.StringVar(&parsed.failedNames, "failed-names", "lint,test-web", "comma-separated failing checks")
	flags.StringVar(&parsed.review, "review", "approved", "review decision")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	required := []string{
		parsed.homeDir,
		parsed.workspaceID,
		parsed.sessionID,
		parsed.branch,
		parsed.repoKey,
	}
	for _, value := range required {
		if value == "" {
			return options{}, errors.New(
				"--home, --workspace, --session, --branch and --repo-key are required",
			)
		}
	}
	if parsed.number <= 0 {
		return options{}, errors.New("--number must be positive")
	}
	return parsed, nil
}
