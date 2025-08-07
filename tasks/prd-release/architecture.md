### Architecture Overview for Release Automation Tool in Go

I'll architect this as a Go CLI tool named `compozy-release` (or similar), designed to replace the bash scripts while maintaining their functionality. The tool will leverage **git-cliff** for changelog generation and version calculation, and **GoReleaser** for the final release process (e.g., building binaries, creating GitHub releases). We'll follow **Clean Architecture** principles adapted for Go:

- **Entities (Domain Layer)**: Core models independent of external concerns (e.g., Version, ReleaseConfig).
- **Use Cases (Application Layer)**: Business logic orchestrating operations (e.g., CalculateVersionUseCase).
- **Interfaces/Adapters (Infrastructure Layer)**: Abstractions for external systems (e.g., GitRepository, FileSystem, ExternalToolExecutor).
- **Frameworks/Drivers (Presentation Layer)**: CLI entry points using Cobra, configuration via Viper.

This ensures:

- **Separation of Concerns**: Business logic doesn't depend on external tools or I/O.
- **Testability**: Interfaces allow mocking (e.g., mock Git, FS, external commands).
- **Extensibility**: Easy to swap implementations (e.g., replace go-git with libgit2 if needed).
- **Best Go Practices**:
  - Idiomatic Go: Use interfaces, errors.Is/wrap, context.Context for cancellation.
  - Dependency Injection: Wire dependencies via constructors.
  - Logging: Structured with zap.
  - Error Handling: Use pkg/errors for wrapping.
  - Concurrency: Safe where needed (e.g., parallel NPM publishes).
  - Performance: Use goccy/go-json for faster JSON ops.
  - Cross-Platform: Handle Windows/macOS/Linux paths via afero.

#### Key Assumptions

- The tool runs in a Git repo context.
- External deps: git-cliff and goreleaser must be installed (or we'll check/fail gracefully).
- NPM publishing uses `npm` CLI via os/exec (idempotent, skips if version exists).
- GitHub integration for PRs, branches, tags.
- Configurable via Viper (e.g., .compozy-release.yaml for paths like "tools/" dir).
- Build as a single binary for easy distribution.

#### Package Structure

```
compozy-release/
├── cmd/
│   └── release/
│       ├── root.go          // Cobra root command setup
│       ├── check-changes.go // Subcommand for check-changes
│       ├── calculate-version.go // And one for each script...
│       └── ...              // e.g., generate-changelog.go, etc.
├── internal/
│   ├── config/              // Viper config loading
│   │   └── config.go
│   ├── domain/              // Entities
│   │   ├── version.go       // Semver wrapper
│   │   ├── release.go       // Release metadata
│   │   └── package.go       // NPM package model
│   ├── usecases/            // Business logic
│   │   ├── check_changes.go // CheckChangesUseCase
│   │   ├── calculate_version.go // And one per feature
│   │   └── ...
│   ├── repositories/        // Interfaces + impls for data access
│   │   ├── git/             // Git ops (using go-git)
│   │   │   └── git.go
│   │   ├── github/          // GitHub API (using google/go-github)
│   │   │   └── github.go
│   │   ├── filesystem/      // FS ops (using afero)
│   │   │   └── fs.go
│   │   └── npm/             // NPM registry checks (using resty)
│   │       └── npm.go
│   ├── services/            // Wrappers for external tools
│   │   ├── cliff/           // git-cliff executor (os/exec)
│   │   │   └── cliff.go
│   │   ├── goreleaser/      // goreleaser executor (os/exec)
│   │   │   └── goreleaser.go
│   │   ├── npm_publisher/   // npm publish wrapper (os/exec + resty)
│   │   │   └── npm.go
│   │   └── template/        // text/template processor
│   │       └── template.go
│   └── logging/             // Zap setup
│       └── logger.go
├── main.go                  // Entry point: executes cmd/root
├── go.mod                   // Dependencies
├── go.sum
└── .compozy-release.yaml    // Example config (optional)
```

#### Dependencies (go.mod)

```go
module github.com/yourorg/compozy-release

go 1.22

require (
    github.com/Masterminds/semver/v3 v3.2.1
    github.com/go-git/go-git/v5 v5.12.0
    github.com/go-resty/resty/v2 v2.14.0
    github.com/goccy/go-json v0.10.3
    github.com/google/go-github/v74 v74.0.0 // Latest as of 2025
    github.com/spf13/afero v1.11.0
    github.com/spf13/cobra v1.8.1
    github.com/spf13/viper v1.19.0
    go.uber.org/zap v1.27.0
    golang.org/x/text v0.18.0 // For template funcs
    github.com/pkg/errors v0.9.1 // Error wrapping
)
```

#### Core Components

1. **Entities (internal/domain)**:
   - `Version`: Wraps semver.Version for validation/comparison/bumping.

     ```go
     type Version struct {
         semver.Version
     }

     func NewVersion(s string) (*Version, error) {
         v, err := semver.NewVersion(s)
         if err != nil {
             return nil, errors.Wrap(err, "invalid version")
         }
         return &Version{*v}, nil
     }

     // Methods: BumpMajor(), BumpMinor(), BumpPatch(), String() (with/without 'v')
     ```

   - `Release`: Holds version, changelog, branch name, etc.
   - `NpmPackage`: Name, Version, Private bool, Dir string.

2. **Use Cases (internal/usecases)**:
   - Each maps to a script, e.g., `CheckChangesUseCase`.
     - Dependencies: Injected repos/services (e.g., GitRepo, CliffService).
     - Example: `check_changes.go`

       ```go
       type CheckChangesUseCase struct {
           gitRepo repositories.GitRepository
           cliffSvc services.CliffService
           logger *zap.Logger
       }

       func (uc *CheckChangesUseCase) Execute(ctx context.Context) (bool, string, error) {
           latestTag, err := uc.gitRepo.LatestTag(ctx)
           if err != nil {
               return false, "", err
           }
           if latestTag == "" {
               return true, "", nil // Initial release
           }

           commitsSince, err := uc.gitRepo.CommitsSinceTag(ctx, latestTag)
           if err != nil || commitsSince == 0 {
               return false, latestTag, err
           }

           // Use git-cliff to check if bump needed
           nextVer, err := uc.cliffSvc.CalculateNextVersion(ctx, latestTag)
           if err != nil {
               return false, latestTag, err
           }
           hasChanges := nextVer.Compare(semver.MustParse(latestTag)) > 0
           return hasChanges, latestTag, nil
       }
       ```

     - Similar for others: CalculateVersion (use cliff to bump), GenerateChangelog (call cliff with mode), etc.
     - For UpdatePackageVersions: Scan "tools/" dir via FS repo, parse JSON, update .Version, write atomically.
     - For PublishNpmPackages: Parallel publish with idempotency (check via NPM repo if version exists).

3. **Repositories (internal/repositories)**:
   - Interfaces for abstraction.
     - `GitRepository`: Impl uses go-git.
       ```go
       type GitRepository interface {
           LatestTag(ctx context.Context) (string, error)
           TagExists(ctx context.Context, tag string) (bool, error)
           CreateBranch(ctx context.Context, name string) error
           CreateTag(ctx context.Context, tag string, msg string) error
           PushTag(ctx context.Context, tag string) error
           // ...
       }
       ```
     - `GithubRepository`: For PR creation, branch checks (using google/go-github + oauth2 token from config).
     - `FileSystem`: Wraps afero.Fs (e.g., ReadDir, WriteFile atomic via temp files).
     - `NpmRepository`: Check version exists via resty GET to registry.npmjs.org.

4. **Services (internal/services)**:
   - `CliffService`: Wrap os/exec for git-cliff.

     ```go
     type CliffService struct{}

     func (s *CliffService) GenerateChangelog(ctx context.Context, version string, mode string) (string, error) {
         args := []string{"--unreleased", "--tag", version}
         if mode == "release" {
             args = append(args, "--current")
         }
         cmd := exec.CommandContext(ctx, "git-cliff", args...)
         output, err := cmd.Output()
         if err != nil {
             return "", errors.Wrap(err, "git-cliff failed")
         }
         return string(output), nil
     }

     // Similar for bump calculation
     ```

   - `GoReleaserService`: Exec "goreleaser release --clean" (integrate in a "release" use case).
   - `NpmPublisher`: Exec "npm publish" per dir, with dry-run flag.
   - `TemplateService`: Process text/template for PR body.
     ```go
     func (s *TemplateService) RenderPRBody(tmplPath string, data map[string]any) (string, error) {
         tmpl, err := template.ParseFiles(tmplPath)
         if err != nil {
             return "", err
         }
         var buf bytes.Buffer
         if err := tmpl.Execute(&buf, data); err != nil {
             return "", err
         }
         return buf.String(), nil
     }
     ```

5. **CLI (cmd/release)**:
   - Root command with subcommands matching scripts.
     - Example: `check-changes.go`
       ```go
       func NewCheckChangesCmd(uc *usecases.CheckChangesUseCase) *cobra.Command {
           cmd := &cobra.Command{
               Use:   "check-changes",
               Short: "Check if changes warrant a release",
               RunE: func(cmd *cobra.Command, args []string) error {
                   hasChanges, latestTag, err := uc.Execute(cmd.Context())
                   if err != nil {
                       return err
                   }
                   fmt.Printf("has_changes=%t\nlatest_tag=%s\n", hasChanges, latestTag)
                   return nil
               },
           }
           return cmd
       }
       ```
     - Outputs match scripts (e.g., "key=value" for GitHub Actions compatibility).
     - Flags: e.g., --dry-run for publish, --mode for generate-changelog.
   - Config: Viper loads from .compozy-release.yaml/env (e.g., github_token, npm_token, tools_dir: "tools").

6. **Main (main.go)**:

   ```go
   func main() {
       logger := logging.NewLogger() // Zap
       defer logger.Sync()

       cfg := config.LoadConfig(logger) // Viper

       // Wire dependencies (manual DI or use wire if complex)
       fs := repositories.NewFileSystem(afero.NewOsFs())
       gitRepo := repositories.NewGitRepository()
       // ...

       rootCmd := cmd.NewRootCmd(/* inject use cases */)
       if err := rootCmd.Execute(); err != nil {
           logger.Fatal("execution failed", zap.Error(err))
       }
   }
   ```

#### Integration with Existing Tools

- **git-cliff**: Used via services for changelog/version calc. Fallback to manual commit parsing if not installed.
- **GoReleaser**: Called in a "release-final" subcommand (not in scripts, but add for completeness).
- **Idempotency**: Check tag/branch exists before create; skip NPM if version published.
- **GitHub Actions**: Outputs parsable as "key=value"; use in workflows like current YAML.

#### Testing Strategy

- Unit: Mock interfaces (e.g., testify/mock for GitRepo).
- Integration: Use temp Git repos (go-git plaintext), afero MemMapFs.
- E2E: Run CLI in test subprocess.

This architecture scales well, is maintainable, and aligns with the scripts' functionality. If you need code for a specific part, let me know!
