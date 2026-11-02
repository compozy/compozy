// Command sessions-board-seeder writes stopped sessions for the Sessions board
// browser journeys (E2E-005..E2E-007) into a stopped isolated daemon home. Each
// one gets what a real stopped session leaves behind — its catalog row and its
// on-disk metadata — so boot reconciliation keeps it instead of orphaning it.
// Their board column comes from the daemon's own derivation: plain rows land in
// Done, a failed child lands in Needs you.
package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type options struct {
	homeDir, workspaceID, agentName, prefix string
	done, archived                          int
	family                                  bool
}

type seeded struct {
	Done     []string `json:"done"`
	Archived []string `json:"archived"`
	Parent   string   `json:"parent,omitempty"`
	Children []string `json:"children,omitempty"`
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

	result, err := seedRows(ctx, database, home.SessionsDir, parsed)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode seeded ids: %w", err)
	}
	if _, err := os.Stdout.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("print seeded ids: %w", err)
	}
	return nil
}

func seedRows(
	ctx context.Context,
	database *globaldb.GlobalDB,
	sessionsDir string,
	parsed options,
) (seeded, error) {
	base := time.Now().UTC().Add(-time.Hour)
	result := seeded{Done: []string{}, Archived: []string{}}
	register := func(info store.SessionInfo) error {
		if err := database.RegisterSession(ctx, &info); err != nil {
			return fmt.Errorf("register seeded session %q: %w", info.ID, err)
		}
		return writeMeta(sessionsDir, &info)
	}
	row := func(id, name string, at time.Time) store.SessionInfo {
		return store.SessionInfo{
			ID:            id,
			ProfileID:     store.DefaultProfileID,
			Name:          name,
			AgentName:     parsed.agentName,
			Provider:      "claude",
			WorkspaceID:   parsed.workspaceID,
			SessionType:   "user",
			State:         "stopped",
			RuntimeStatus: store.SessionRuntimeUnbound,
			StopReason:    store.StopCompleted,
			CreatedAt:     at,
			UpdatedAt:     at,
		}
	}
	for index := range parsed.done {
		id := fmt.Sprintf("%s-done-%03d", parsed.prefix, index)
		at := base.Add(time.Duration(index) * time.Second)
		if err := register(row(id, fmt.Sprintf("Done session %03d", index), at)); err != nil {
			return seeded{}, err
		}
		result.Done = append(result.Done, id)
	}
	for index := range parsed.archived {
		id := fmt.Sprintf("%s-archived-%d", parsed.prefix, index)
		at := base.Add(time.Duration(index) * time.Second)
		if err := register(row(id, fmt.Sprintf("Archived session %d", index), at)); err != nil {
			return seeded{}, err
		}
		// Registration does not write archive state; the archive owner does.
		if _, err := database.SetSessionArchived(ctx, parsed.workspaceID, id, true); err != nil {
			return seeded{}, fmt.Errorf("archive seeded session %q: %w", id, err)
		}
		result.Archived = append(result.Archived, id)
	}
	if parsed.family {
		fail := func(id string) error {
			// Registration does not write failure facts; the state update does.
			if err := database.UpdateSessionState(ctx, store.SessionStateUpdate{
				ID:            id,
				State:         "stopped",
				RuntimeStatus: store.SessionRuntimeUnbound,
				FailureSet:    true,
				Failure:       &store.SessionFailure{Kind: store.FailurePrompt, Summary: "seeded failure"},
			}); err != nil {
				return fmt.Errorf("fail seeded session %q: %w", id, err)
			}
			return nil
		}
		parent, children, err := seedFamily(base, parsed.prefix, row, register, fail)
		if err != nil {
			return seeded{}, err
		}
		result.Parent, result.Children = parent, children
	}
	return result, nil
}

// seedFamily writes a coordinator and three direct children; the first child
// failed, so it needs the operator and lights the parent's rollup.
func seedFamily(
	base time.Time,
	prefix string,
	row func(id, name string, at time.Time) store.SessionInfo,
	register func(store.SessionInfo) error,
	fail func(id string) error,
) (string, []string, error) {
	parentID := prefix + "-parent"
	if err := register(row(parentID, "Board coordinator", base.Add(time.Hour))); err != nil {
		return "", nil, err
	}
	children := make([]string, 0, 3)
	for index := range 3 {
		id := fmt.Sprintf("%s-child-%d", prefix, index)
		info := row(id, fmt.Sprintf("Board worker %d", index), base.Add(time.Hour+time.Duration(index+1)*time.Second))
		info.SessionType = "spawned"
		info.Lineage = &store.SessionLineage{
			ParentSessionID: parentID,
			RootSessionID:   parentID,
			SpawnDepth:      1,
			SpawnRole:       "worker",
			Kind:            store.LineageKindSpawn,
		}
		if index == 0 {
			info.Failure = &store.SessionFailure{Kind: store.FailurePrompt, Summary: "seeded failure"}
		}
		if err := register(info); err != nil {
			return "", nil, err
		}
		if index == 0 {
			if err := fail(id); err != nil {
				return "", nil, err
			}
		}
		children = append(children, id)
	}
	return parentID, children, nil
}

func parseOptions(args []string) (options, error) {
	var parsed options
	flags := flag.NewFlagSet("sessions-board-seeder", flag.ContinueOnError)
	flags.StringVar(&parsed.homeDir, "home", "", "isolated COMPOZY_HOME")
	flags.StringVar(&parsed.workspaceID, "workspace", "", "workspace id owning the rows")
	flags.StringVar(&parsed.agentName, "agent", "", "agent name recorded on every row")
	flags.StringVar(&parsed.prefix, "prefix", "sess-board", "session id prefix")
	flags.IntVar(&parsed.done, "done", 0, "stopped sessions to seed into Done")
	flags.IntVar(&parsed.archived, "archived", 0, "archived sessions to seed")
	flags.BoolVar(&parsed.family, "family", false, "seed a parent with three children, one failed")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if parsed.homeDir == "" || parsed.workspaceID == "" || parsed.agentName == "" {
		return options{}, errors.New("--home, --workspace and --agent are required")
	}
	return parsed, nil
}

// writeMeta records the session directory boot reconciliation recovers from.
func writeMeta(sessionsDir string, info *store.SessionInfo) error {
	stopReason := info.StopReason
	meta := store.SessionMeta{
		ID:            info.ID,
		Name:          info.Name,
		AgentName:     info.AgentName,
		Provider:      info.Provider,
		RuntimeStatus: info.RuntimeStatus,
		ProfileID:     info.ProfileID,
		WorkspaceID:   info.WorkspaceID,
		SessionType:   info.SessionType,
		Lineage:       info.Lineage,
		State:         info.State,
		StopReason:    &stopReason,
		Failure:       info.Failure,
		CreatedAt:     info.CreatedAt,
		UpdatedAt:     info.UpdatedAt,
	}
	path := store.SessionMetaFile(filepath.Join(sessionsDir, info.ID))
	if err := store.WriteSessionMeta(path, &meta); err != nil {
		return fmt.Errorf("write seeded session metadata %q: %w", info.ID, err)
	}
	return nil
}
