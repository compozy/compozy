package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkspaceAddBuildsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		request WorkspaceCreateRequest
	}{
		{
			name: "minimal",
			args: []string{"workspace", "add", "/workspace/project", "-o", "json"},
			request: WorkspaceCreateRequest{
				RootDir: "/workspace/project",
			},
		},
		{
			name: "with optional flags",
			args: []string{
				"workspace", "add", "/workspace/project",
				"--name", "alpha",
				"--add-dir", "/workspace/shared-a",
				"--add-dir", "/workspace/shared-b",
				"--default-agent", "coder",
				"--sandbox", "daytona-dev",
				"-o", "json",
			},
			request: WorkspaceCreateRequest{
				RootDir:      "/workspace/project",
				Name:         "alpha",
				AddDirs:      []string{"/workspace/shared-a", "/workspace/shared-b"},
				DefaultAgent: "coder",
				SandboxRef:   "daytona-dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				createWorkspaceFn: func(_ context.Context, request WorkspaceCreateRequest) (WorkspaceRecord, error) {
					if request.RootDir != tt.request.RootDir || request.Name != tt.request.Name ||
						request.DefaultAgent != tt.request.DefaultAgent ||
						request.SandboxRef != tt.request.SandboxRef {
						t.Fatalf("CreateWorkspace() request = %#v, want %#v", request, tt.request)
					}
					if strings.Join(request.AddDirs, ",") != strings.Join(tt.request.AddDirs, ",") {
						t.Fatalf("CreateWorkspace() AddDirs = %#v, want %#v", request.AddDirs, tt.request.AddDirs)
					}
					return WorkspaceRecord{
						ID:           "ws_alpha",
						RootDir:      request.RootDir,
						AddDirs:      request.AddDirs,
						Name:         firstNonEmpty(request.Name, "alpha"),
						DefaultAgent: request.DefaultAgent,
						SandboxRef:   request.SandboxRef,
						CreatedAt:    fixedTestNow,
						UpdatedAt:    fixedTestNow,
					}, nil
				},
			})

			stdout, _, err := executeRootCommand(t, deps, tt.args...)
			if err != nil {
				t.Fatalf("executeRootCommand(%v) error = %v", tt.args, err)
			}

			var decoded WorkspaceRecord
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(workspace add) error = %v", err)
			}
			if decoded.ID != "ws_alpha" {
				t.Fatalf("decoded.ID = %q, want %q", decoded.ID, "ws_alpha")
			}
		})
	}
}

func TestWorkspaceEditBuildsRequest(t *testing.T) {
	t.Parallel()

	t.Run("Should build request", func(t *testing.T) {
		t.Parallel()

		var (
			seenRef     string
			seenRequest WorkspaceUpdateRequest
		)

		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, _ string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{
						ID:      "ws_alpha",
						RootDir: "/workspace/project",
						AddDirs: []string{"/workspace/shared-a", "/workspace/shared-b"},
						Name:    "alpha",
					},
				}, nil
			},
			updateWorkspaceFn: func(_ context.Context, ref string, request WorkspaceUpdateRequest) (WorkspaceRecord, error) {
				seenRef = ref
				seenRequest = request
				return WorkspaceRecord{
					ID:           "ws_alpha",
					RootDir:      "/workspace/project",
					AddDirs:      derefStringSlice(request.AddDirs),
					Name:         derefString(request.Name),
					DefaultAgent: derefString(request.DefaultAgent),
					SandboxRef:   derefString(request.SandboxRef),
					CreatedAt:    fixedTestNow,
					UpdatedAt:    fixedTestNow,
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps,
			"workspace", "edit", "alpha",
			"--name", "beta",
			"--add-dir", "/workspace/shared-c",
			"--remove-dir", "/workspace/shared-a",
			"--default-agent", "reviewer",
			"--sandbox", "local-dev",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("executeRootCommand(workspace edit) error = %v", err)
		}
		if seenRef != "ws_alpha" {
			t.Fatalf("UpdateWorkspace() ref = %q, want %q", seenRef, "ws_alpha")
		}
		if seenRequest.Name == nil || *seenRequest.Name != "beta" {
			t.Fatalf("UpdateWorkspace() Name = %#v, want beta", seenRequest.Name)
		}
		if seenRequest.AddDirs == nil ||
			strings.Join(*seenRequest.AddDirs, ",") != "/workspace/shared-b,/workspace/shared-c" {
			t.Fatalf("UpdateWorkspace() AddDirs = %#v, want filtered/appended dirs", seenRequest.AddDirs)
		}
		if seenRequest.DefaultAgent == nil || *seenRequest.DefaultAgent != "reviewer" {
			t.Fatalf("UpdateWorkspace() DefaultAgent = %#v, want reviewer", seenRequest.DefaultAgent)
		}
		if seenRequest.SandboxRef == nil || *seenRequest.SandboxRef != "local-dev" {
			t.Fatalf("UpdateWorkspace() SandboxRef = %#v, want local-dev", seenRequest.SandboxRef)
		}

		var decoded WorkspaceRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(workspace edit) error = %v", err)
		}
		if decoded.Name != "beta" {
			t.Fatalf("decoded.Name = %q, want %q", decoded.Name, "beta")
		}
	})
}

func TestWorkspaceEditRejectsConflictingDirUpdates(t *testing.T) {
	t.Parallel()

	t.Run("Should reject conflicting dir updates", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, _ string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{ID: "ws_alpha", AddDirs: []string{"/workspace/shared-a"}},
				}, nil
			},
		})

		code, _, stderr := executeRootCommandWithExit(t, deps,
			"workspace", "edit", "alpha",
			"--add-dir", "/workspace/shared-a",
			"--remove-dir", "/workspace/shared-a",
		)
		if code != 1 {
			t.Fatalf("executeRootCommandWithExit() code = %d, want 1", code)
		}
		if !strings.Contains(stderr, "cannot add and remove the same directory") {
			t.Fatalf("stderr = %q, want conflicting directory validation message", stderr)
		}
	})
}

func TestWorkspaceListInfoAndRemove(t *testing.T) {
	t.Parallel()

	t.Run("Should list info and remove", func(t *testing.T) {
		t.Parallel()

		var deletedRef string

		deps := newTestDeps(t, &stubClient{
			listWorkspacesFn: func(context.Context) ([]WorkspaceRecord, error) {
				return []WorkspaceRecord{{
					ID:        "ws_alpha",
					RootDir:   "/workspace/project",
					Name:      "alpha",
					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				}}, nil
			},
			getWorkspaceFn: func(_ context.Context, _ string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{
						ID:        "ws_alpha",
						RootDir:   "/workspace/project",
						Name:      "alpha",
						CreatedAt: fixedTestNow,
						UpdatedAt: fixedTestNow,
					},
					Sessions: []SessionRecord{{
						ID:            "sess-1",
						AgentName:     "coder",
						WorkspaceID:   "ws_alpha",
						WorkspacePath: "/workspace/project",
						State:         "active",
						CreatedAt:     fixedTestNow,
						UpdatedAt:     fixedTestNow,
					}},
					Agents: []AgentRecord{{
						Name:         "coder",
						Provider:     "fake",
						CategoryPath: []string{"Engineering", "Tools"},
						Prompt:       "hi",
					}},
					Skills: []WorkspaceSkillRecord{{
						Name:   "review",
						Dir:    "/workspace/project/.agh/skills/review",
						Source: "workspace",
					}},
				}, nil
			},
			deleteWorkspaceFn: func(_ context.Context, ref string) error {
				deletedRef = ref
				return nil
			},
		})

		listOut, _, err := executeRootCommand(t, deps, "workspace", "list", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace list) error = %v", err)
		}
		var listed []WorkspaceRecord
		if err := json.Unmarshal([]byte(listOut), &listed); err != nil {
			t.Fatalf("json.Unmarshal(workspace list) error = %v", err)
		}
		if len(listed) != 1 || listed[0].ID != "ws_alpha" {
			t.Fatalf("listed = %#v, want ws_alpha", listed)
		}

		infoOut, _, err := executeRootCommand(t, deps, "workspace", "info", "alpha", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace info) error = %v", err)
		}
		var detail WorkspaceDetailRecord
		if err := json.Unmarshal([]byte(infoOut), &detail); err != nil {
			t.Fatalf("json.Unmarshal(workspace info) error = %v", err)
		}
		if detail.Workspace.ID != "ws_alpha" || len(detail.Sessions) != 1 || len(detail.Agents) != 1 ||
			len(detail.Skills) != 1 {
			t.Fatalf("detail = %#v, want workspace detail payload", detail)
		}
		if got, want := strings.Join(detail.Agents[0].CategoryPath, ","), "Engineering,Tools"; got != want {
			t.Fatalf("detail.Agents[0].CategoryPath = %#v, want %q", detail.Agents[0].CategoryPath, want)
		}

		removeOut, _, err := executeRootCommand(t, deps, "workspace", "remove", "alpha", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace remove) error = %v", err)
		}
		if deletedRef != "ws_alpha" {
			t.Fatalf("DeleteWorkspace() ref = %q, want %q", deletedRef, "ws_alpha")
		}

		var removed WorkspaceRecord
		if err := json.Unmarshal([]byte(removeOut), &removed); err != nil {
			t.Fatalf("json.Unmarshal(workspace remove) error = %v", err)
		}
		if removed.ID != "ws_alpha" {
			t.Fatalf("removed.ID = %q, want %q", removed.ID, "ws_alpha")
		}
	})
}

func TestWorkspaceInfoResolvesReferenceSources(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		envValue   string
		cwd        string
		wantRef    string
		wantSource string
	}{
		{
			name:       "Should prefer positional ref over flag and env",
			args:       []string{"workspace", "info", "ws-alpha", "--workspace", "/workspace/beta", "-o", "json"},
			envValue:   "ws-env",
			cwd:        "/workspace/alpha",
			wantRef:    "ws-alpha",
			wantSource: "positional",
		},
		{
			name:       "Should use workspace flag when positional is omitted",
			args:       []string{"workspace", "info", "--workspace", "/workspace/beta", "-o", "json"},
			envValue:   "ws-env",
			cwd:        "/workspace/alpha",
			wantRef:    "/workspace/beta",
			wantSource: "flag",
		},
		{
			name:       "Should use AGH_WORKSPACE when flag and positional are omitted",
			args:       []string{"workspace", "info", "-o", "json"},
			envValue:   "ws-beta",
			cwd:        "/workspace/alpha",
			wantRef:    "ws-beta",
			wantSource: "env",
		},
		{
			name:       "Should fall back to cwd",
			args:       []string{"workspace", "info", "-o", "json"},
			cwd:        "/workspace/alpha",
			wantRef:    "/workspace/alpha",
			wantSource: "cwd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var seenRef string
			deps := newTestDeps(t, &stubClient{
				getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
					seenRef = ref
					return WorkspaceDetailRecord{
						Workspace: WorkspaceRecord{
							ID:      "ws_alpha",
							RootDir: ref,
							Name:    "alpha",
						},
					}, nil
				},
			})
			deps.getwd = func() (string, error) {
				return tt.cwd, nil
			}
			deps.getenv = func(key string) string {
				if key == "AGH_WORKSPACE" {
					return tt.envValue
				}
				return ""
			}

			stdout, _, err := executeRootCommand(t, deps, tt.args...)
			if err != nil {
				t.Fatalf("executeRootCommand(%v) error = %v", tt.args, err)
			}
			if seenRef != tt.wantRef {
				t.Fatalf("GetWorkspace() ref = %q, want %q", seenRef, tt.wantRef)
			}
			var decoded workspaceDetailOutput
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(workspace info) error = %v", err)
			}
			if decoded.ResolutionSource != tt.wantSource {
				t.Fatalf("ResolutionSource = %q, want %q", decoded.ResolutionSource, tt.wantSource)
			}
			if decoded.Workspace.RootDir != tt.wantRef {
				t.Fatalf("Workspace.RootDir = %q, want %q", decoded.Workspace.RootDir, tt.wantRef)
			}
		})
	}
}

func TestWorkspaceOutputFormats(t *testing.T) {
	t.Parallel()

	t.Run("Should render output formats", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listWorkspacesFn: func(context.Context) ([]WorkspaceRecord, error) {
				return []WorkspaceRecord{{
					ID:           "ws_alpha",
					RootDir:      "/workspace/project",
					AddDirs:      []string{"/workspace/shared"},
					Name:         "alpha",
					DefaultAgent: "coder",
					CreatedAt:    fixedTestNow,
					UpdatedAt:    fixedTestNow,
				}}, nil
			},
			getWorkspaceFn: func(_ context.Context, _ string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{
					Workspace: WorkspaceRecord{
						ID:           "ws_alpha",
						RootDir:      "/workspace/project",
						AddDirs:      []string{"/workspace/shared"},
						Name:         "alpha",
						DefaultAgent: "coder",
						CreatedAt:    fixedTestNow,
						UpdatedAt:    fixedTestNow,
					},
					Sessions: []SessionRecord{{
						ID:            "sess-1",
						Name:          "demo",
						AgentName:     "coder",
						WorkspaceID:   "ws_alpha",
						WorkspacePath: "/workspace/project",
						State:         "active",
						CreatedAt:     fixedTestNow,
						UpdatedAt:     fixedTestNow,
					}},
					Agents: []AgentRecord{{
						Name:         "coder",
						Provider:     "fake",
						Model:        "gpt-5.4",
						CategoryPath: []string{"Engineering", "Tools"},
						Permissions:  "approve-reads",
						Prompt:       "hi",
					}},
					Skills: []WorkspaceSkillRecord{{
						Name:   "review",
						Dir:    "/workspace/project/.agh/skills/review",
						Source: "workspace",
					}},
				}, nil
			},
		})

		listHuman, _, err := executeRootCommand(t, deps, "workspace", "list", "-o", "human")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace list human) error = %v", err)
		}
		if !strings.Contains(listHuman, "Workspaces") || !strings.Contains(listHuman, "alpha") {
			t.Fatalf("list human output = %q, want workspace table", listHuman)
		}

		listToon, _, err := executeRootCommand(t, deps, "workspace", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace list toon) error = %v", err)
		}
		if !strings.Contains(
			listToon,
			"workspaces[1]{id,name,root_dir,add_dir_count,default_agent,sandbox_ref,updated_at}:",
		) {
			t.Fatalf("list toon output = %q, want TOON header", listToon)
		}

		infoHuman, _, err := executeRootCommand(t, deps, "workspace", "info", "alpha", "-o", "human")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace info human) error = %v", err)
		}
		if !strings.Contains(infoHuman, "Workspace") || !strings.Contains(infoHuman, "Sessions") ||
			!strings.Contains(infoHuman, "Skills") ||
			!strings.Contains(infoHuman, "Category") ||
			!strings.Contains(infoHuman, "Engineering / Tools") {
			t.Fatalf("info human output = %q, want workspace detail sections", infoHuman)
		}

		infoToon, _, err := executeRootCommand(t, deps, "workspace", "info", "alpha", "-o", "toon")
		if err != nil {
			t.Fatalf("executeRootCommand(workspace info toon) error = %v", err)
		}
		if !strings.Contains(infoToon, "skills[1]{name,source,dir}:") ||
			!strings.Contains(infoToon, "agents[1]{name,provider,model,category,permissions}:") ||
			!strings.Contains(infoToon, "Engineering / Tools") {
			t.Fatalf("info toon output = %q, want TOON detail blocks", infoToon)
		}
	})
}

func TestWorkspaceEditRequiresChanges(t *testing.T) {
	t.Parallel()

	t.Run("Should require changes", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := executeRootCommandWithExit(t, newTestDeps(t, &stubClient{}), "workspace", "edit", "alpha")
		if code != 1 {
			t.Fatalf("executeRootCommandWithExit() code = %d, want 1", code)
		}
		if !strings.Contains(stderr, "at least one edit flag is required") {
			t.Fatalf("stderr = %q, want edit validation message", stderr)
		}
	})
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefStringSlice(value *[]string) []string {
	if value == nil {
		return nil
	}
	return append([]string(nil), (*value)...)
}
