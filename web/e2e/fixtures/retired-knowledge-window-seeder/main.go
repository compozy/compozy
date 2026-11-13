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

	"github.com/compozy/compozy/internal/clientstate"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/windowmanager"
)

type options struct {
	homeDir, workspaceID, profileID, sessionWindowID, knowledgeWindowID string
	rect                                                                windowmanager.NormalizedRect
}

type workspaceResolver struct{ database *globaldb.GlobalDB }

var _ clientstate.WorkspaceResolver = (*workspaceResolver)(nil)

func (r *workspaceResolver) ResolveWorkspace(
	ctx context.Context,
	id clientstate.WorkspaceID,
) (clientstate.WorkspaceGeneration, error) {
	if id == clientstate.WorkspaceID(windowmanager.GlobalDesktopWorkspaceID) {
		return clientstate.WorkspaceGeneration(id), nil
	}
	workspace, err := r.database.GetWorkspace(ctx, string(id))
	if err != nil {
		return "", fmt.Errorf("resolve registered workspace: %w", err)
	}
	return clientstate.WorkspaceGeneration(workspace.ID + ":" + workspace.CreatedAt.Format(time.RFC3339Nano)), nil
}

func (r *workspaceResolver) ResolveWorkspaceForPurge(
	context.Context,
	clientstate.WorkspaceID,
) (clientstate.WorkspaceGeneration, error) {
	return "", errors.New("retired window seeder does not purge workspaces")
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
		return fmt.Errorf("open isolated global database: %w", err)
	}
	defer func() { err = errors.Join(err, database.Close(ctx)) }()
	limits := clientstate.DefaultLimits()
	limits.MaxValueBytes = 16 * 1024 * 1024
	engine, err := clientstate.Open(
		ctx,
		clientstate.DatabasePath(home.HomeDir),
		&workspaceResolver{database: database},
		limits,
	)
	if err != nil {
		return fmt.Errorf("open raw client state (stop the isolated daemon first): %w", err)
	}
	defer func() { err = errors.Join(err, engine.Close()) }()
	workspaceID := clientstate.WorkspaceID(parsed.workspaceID)
	key := "snapshot:" + parsed.profileID
	entry, err := engine.Get(ctx, workspaceID, "window_manager", key)
	if err != nil {
		return fmt.Errorf("read raw stored window snapshot: %w", err)
	}
	snapshot, err := legacySnapshotForSeed(entry.Value)
	if err != nil {
		return err
	}
	if err := addRetiredKnowledgeWindow(&snapshot, parsed); err != nil {
		return err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encode seeded snapshot: %w", err)
	}
	if _, err := engine.Apply(ctx, workspaceID, "window_manager", []clientstate.Op{
		{Kind: clientstate.OpPut, Key: key, Value: encoded, IfRev: entry.Rev},
	}, clientstate.ApplyOptions{Origin: "e2e.retired-knowledge-window"}); err != nil {
		return fmt.Errorf("persist raw seeded snapshot: %w", err)
	}
	stored, err := engine.Get(ctx, workspaceID, "window_manager", key)
	if err != nil {
		return fmt.Errorf("reread raw seeded snapshot: %w", err)
	}
	if _, err := os.Stdout.Write(append(stored.Value, '\n')); err != nil {
		return fmt.Errorf("print raw seeded snapshot: %w", err)
	}
	return nil
}

// Seed the previous release's persisted shape so cold boot exercises both the
// v5 upgrade and retired-app reconciliation. Shared content and config survive;
// v6 history is reset because its states cannot be represented as v5 history.
func legacySnapshotForSeed(encoded []byte) (windowmanager.SnapshotV5, error) {
	var current windowmanager.Snapshot
	if err := json.Unmarshal(encoded, &current); err != nil {
		return windowmanager.SnapshotV5{}, fmt.Errorf("decode current snapshot: %w", err)
	}
	if err := windowmanager.ValidateSnapshot(current); err != nil {
		return windowmanager.SnapshotV5{}, fmt.Errorf("validate current snapshot: %w", err)
	}
	var legacy windowmanager.SnapshotV5
	if err := json.Unmarshal(encoded, &legacy); err != nil {
		return windowmanager.SnapshotV5{}, fmt.Errorf("copy shared snapshot fields: %w", err)
	}
	legacy.Version = 5
	legacy.History = []byte(`{"undo":[],"redo":[]}`)
	legacy.Desktops = make([]windowmanager.DesktopV5, 0, len(current.Desktops))
	pins := make(map[windowmanager.TabID]bool)
	for _, desktop := range current.Desktops {
		previous := windowmanager.DesktopV5{
			ID: desktop.ID, Name: desktop.Name, Order: desktop.Order,
			Groups: []windowmanager.LayoutGroupV5{}, Floating: []windowmanager.WindowID{},
			FloatingStacks: []windowmanager.FloatingStackV5{},
		}
		// v5 has no durable active tab. Put its group first so conversion keeps
		// the currently selected session visible after the retired app is removed.
		for _, active := range []bool{true, false} {
			for _, tab := range desktop.Tabs {
				if (tab.ID == desktop.ActiveTabID) != active {
					continue
				}
				pins[tab.ID] = tab.Pinned
				previous.Groups = append(previous.Groups, windowmanager.LayoutGroupV5{
					ID:    windowmanager.GroupID(tab.ID),
					Frame: windowmanager.NormalizedRect{Width: 1, Height: 1},
					Root:  legacyLayoutNode(tab.Root),
				})
			}
		}
		legacy.Desktops = append(legacy.Desktops, previous)
	}
	for id, window := range legacy.Windows {
		window.Placement = windowmanager.WindowPlacementTiled
		window.Pinned = pins[current.Windows[id].TabID]
		legacy.Windows[id] = window
	}
	return legacy, nil
}

func legacyLayoutNode(node windowmanager.LayoutNode) windowmanager.LayoutNodeV5 {
	children := make([]windowmanager.LayoutNodeV5, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, legacyLayoutNode(child))
	}
	return windowmanager.LayoutNodeV5{
		ID: node.ID, Kind: node.Kind, WindowID: node.WindowID, Axis: node.Axis,
		Children: children, Weights: node.Weights,
	}
}

func addRetiredKnowledgeWindow(snapshot *windowmanager.SnapshotV5, parsed options) error {
	sessionID := windowmanager.WindowID(parsed.sessionWindowID)
	session, exists := snapshot.Windows[sessionID]
	if !exists || session.App != "session" {
		return fmt.Errorf("snapshot requires registered session window %q", sessionID)
	}
	knowledgeID := windowmanager.WindowID(parsed.knowledgeWindowID)
	if _, exists := snapshot.Windows[knowledgeID]; exists {
		return fmt.Errorf("knowledge window %q already exists", knowledgeID)
	}
	snapshot.Windows[knowledgeID] = windowmanager.WindowV5{
		ID: knowledgeID, App: "knowledge", DesktopID: session.DesktopID,
		Placement: windowmanager.WindowPlacementFloating, FloatingRect: parsed.rect,
		Route:    windowmanager.RouteIntent{Pathname: "/knowledge", Search: windowmanager.RouteSearch{}},
		NavStack: []windowmanager.RouteIntent{},
	}
	foundDesktop := false
	for index := range snapshot.Desktops {
		if snapshot.Desktops[index].ID == session.DesktopID {
			snapshot.Desktops[index].Floating = append(
				[]windowmanager.WindowID{knowledgeID},
				snapshot.Desktops[index].Floating...)
			foundDesktop = true
			break
		}
	}
	if !foundDesktop {
		return fmt.Errorf("session desktop %q is missing", session.DesktopID)
	}
	revision, err := windowmanager.NextTopologyRevision(snapshot.Revision)
	if err != nil {
		return err
	}
	snapshot.Revision = revision
	snapshot.UpdatedAt = time.Now().UTC()
	if _, _, err := windowmanager.ConvertSnapshotV5(*snapshot); err != nil {
		return fmt.Errorf("validate seeded topology: %w", err)
	}
	return nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("retired-knowledge-window-seeder", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var parsed options
	flags.StringVar(&parsed.homeDir, "home", "", "isolated CompozyOS home")
	flags.StringVar(&parsed.workspaceID, "workspace", "", "workspace id")
	flags.StringVar(&parsed.profileID, "profile", store.DefaultProfileID, "owning profile id")
	flags.StringVar(&parsed.sessionWindowID, "session-window", "", "existing registered session window id")
	flags.StringVar(&parsed.knowledgeWindowID, "knowledge-window", "", "retired knowledge window id")
	flags.Float64Var(&parsed.rect.X, "x", 0.55, "knowledge floating x")
	flags.Float64Var(&parsed.rect.Y, "y", 0.1, "knowledge floating y")
	flags.Float64Var(&parsed.rect.Width, "width", 0.4, "knowledge floating width")
	flags.Float64Var(&parsed.rect.Height, "height", 0.7, "knowledge floating height")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if strings.TrimSpace(parsed.homeDir) == "" || strings.TrimSpace(parsed.workspaceID) == "" ||
		strings.TrimSpace(parsed.profileID) == "" || strings.TrimSpace(parsed.sessionWindowID) == "" ||
		strings.TrimSpace(parsed.knowledgeWindowID) == "" {
		return options{}, errors.New(
			"retired knowledge window seeder requires home, workspace, session-window, and knowledge-window",
		)
	}
	return parsed, nil
}
