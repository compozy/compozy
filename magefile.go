//go:build mage

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"github.com/magefile/mage/target"
)

// Default target runs when no target is specified
var Default = Dev

const (
	binaryName = "compozy"
	binaryDir  = "bin"
	swaggerDir = "docs"
	goVersion  = "1.25"
)

var (
	mainPackages = []string{"cli", "engine", "pkg", "test"}
	// Build variables
	gitCommit = getGitCommit()
	version   = getVersion()
	buildDate = time.Now().UTC().Format("2006-01-02T15:04:05Z")
)

// Quality contains all code quality targets
type Quality mg.Namespace

// Docker contains all docker-related targets
type Docker mg.Namespace

// Database contains all database-related targets
type Database mg.Namespace

// Redis contains all redis-related targets
type Redis mg.Namespace

// Schema contains schema generation targets
type Schema mg.Namespace

// Integration groups integration test targets
type Integration mg.Namespace

// Dev runs the development server with hot reload
func Dev() error {
	example := os.Getenv("EXAMPLE")
	if example == "" {
		example = "weather"
	}
	return sh.RunV("gow", "run", ".", "dev",
		"--cwd", "examples/"+example,
		"--env-file", ".env",
		"--debug",
		"--watch")
}

// Build compiles the binary with version info
func Build(ctx context.Context) error {
	mg.CtxDeps(ctx, checkGoVersion, Swagger)
	if err := os.MkdirAll(binaryDir, 0755); err != nil {
		return err
	}
	ldflags := fmt.Sprintf(
		"-X github.com/compozy/compozy/pkg/version.Version=%s "+
			"-X github.com/compozy/compozy/pkg/version.CommitHash=%s "+
			"-X github.com/compozy/compozy/pkg/version.BuildDate=%s",
		version, gitCommit, buildDate,
	)
	if err := sh.RunV("go", "build", "-ldflags", ldflags, "-o", filepath.Join(binaryDir, binaryName), "./"); err != nil {
		return err
	}
	return sh.RunV("chmod", "+x", filepath.Join(binaryDir, binaryName))
}

// Clean removes build artifacts
func Clean() error {
	fmt.Println("Cleaning build artifacts...")
	if err := sh.Rm(binaryDir); err != nil {
		fmt.Printf("Warning: failed to remove %s: %v\n", binaryDir, err)
	}
	if err := sh.Rm(swaggerDir); err != nil {
		fmt.Printf("Warning: failed to remove %s: %v\n", swaggerDir, err)
	}
	return sh.RunV("go", "clean")
}

// Test runs all test suites in parallel
func Test(ctx context.Context) error {
	fmt.Println("Running all tests in parallel...")
	mg.CtxDeps(ctx, testMain, testSDK, testBun)
	fmt.Println("✓ All tests passed")
	return nil
}

// TestCoverage runs all tests with coverage
func TestCoverage(ctx context.Context) error {
	fmt.Println("Running tests with coverage...")
	mg.CtxDeps(ctx, testCoverageMain, testCoverageSDK, testBun)
	fmt.Println("✓ All tests with coverage completed")
	return nil
}

// TestNoCache runs all tests without cache
func TestNoCache(ctx context.Context) error {
	fmt.Println("Running tests without cache...")
	mg.CtxDeps(ctx, testNoCacheMain, testNoCacheSDK, testBun)
	fmt.Println("✓ All tests (no cache) completed")
	return nil
}

// All runs swagger, tests, linting, and formatting in optimal order
func All(ctx context.Context) error {
	fmt.Println("Running all checks...")
	if err := Swagger(ctx); err != nil {
		return err
	}
	mg.CtxDeps(ctx, Test, Quality.Lint, Quality.Fmt)
	fmt.Println("✓ All checks passed")
	return nil
}

// Setup installs all dependencies and checks Go version
func Setup(ctx context.Context) error {
	mg.CtxDeps(ctx, checkGoVersion, Deps)
	fmt.Println("✓ Setup complete! You can now run 'mage build' or 'mage dev'")
	return nil
}

// Deps installs all required dependencies including mage
func Deps(ctx context.Context) error {
	fmt.Println("Installing Go dependencies...")
	mg.CtxDeps(ctx, cleanGoCache, swaggerDeps)
	deps := []struct {
		name string
		pkg  string
	}{
		{"mage", "github.com/magefile/mage@latest"},
		{"gotestsum", "gotest.tools/gotestsum@latest"},
		{"gow", "github.com/mitranim/gow@latest"},
		{"goose", "github.com/pressly/goose/v3/cmd/goose@latest"},
	}
	for _, dep := range deps {
		fmt.Printf("Installing %s...\n", dep.name)
		if err := sh.RunV("go", "install", dep.pkg); err != nil {
			return fmt.Errorf("failed to install %s: %w", dep.name, err)
		}
	}
	fmt.Println("Installing golangci-lint v2...")
	installScript := "curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.4.0"
	if err := sh.RunV("sh", "-c", installScript); err != nil {
		return fmt.Errorf("failed to install golangci-lint: %w", err)
	}
	fmt.Println("✓ All dependencies installed successfully")
	return nil
}

// Tidy runs go mod tidy
func Tidy() error {
	fmt.Println("Tidying modules...")
	return sh.RunV("go", "mod", "tidy")
}

// Swagger generates API documentation
func Swagger(ctx context.Context) error {
	needsRebuild, err := swaggerNeedsRebuild()
	if err != nil {
		return err
	}
	if !needsRebuild {
		fmt.Println("✓ Swagger documentation is up-to-date")
		return nil
	}
	fmt.Println("Generating Swagger documentation...")
	if err := os.MkdirAll(swaggerDir, 0755); err != nil {
		return err
	}
	cmd := exec.Command("swag", "init",
		"--dir", "./",
		"--generalInfo", "main.go",
		"--output", swaggerDir,
		"--parseDependency",
		"--parseInternal",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("swagger generation failed: %w\n%s", err, output)
	}
	filtered := filterSwaggerWarnings(string(output))
	if filtered != "" {
		fmt.Print(filtered)
	}
	fmt.Println("Running pre-commit on generated swagger files...")
	preCommitFiles := []string{
		filepath.Join(swaggerDir, "docs.go"),
		filepath.Join(swaggerDir, "swagger.json"),
		filepath.Join(swaggerDir, "swagger.yaml"),
	}
	if err := sh.Run("pre-commit", append([]string{"run", "--files"}, preCommitFiles...)...); err != nil {
		fmt.Println("Warning: pre-commit checks failed (continuing anyway)")
	}
	fmt.Printf("Swagger documentation generated at %s\n", swaggerDir)
	return nil
}

// SwaggerValidate validates Swagger documentation
func SwaggerValidate() error {
	fmt.Println("Validating Swagger documentation...")
	return sh.RunV("swag", "init",
		"--dir", "./",
		"--generalInfo", "main.go",
		"--output", swaggerDir,
		"--parseDependency",
		"--parseInternal",
		"--quiet",
	)
}

// Lint runs all linters in parallel
func (Quality) Lint(ctx context.Context) error {
	fmt.Println("Running linters in parallel...")
	mg.CtxDeps(ctx,
		lintMain,
		lintSDK,
		lintBun,
		checkDriverImports,
	)
	fmt.Println("✓ Linting completed successfully")
	return nil
}

// Fmt formats all code in parallel
func (Quality) Fmt(ctx context.Context) error {
	fmt.Println("Formatting code in parallel...")
	mg.CtxDeps(ctx, fmtMain, fmtSDK, fmtBun)
	fmt.Println("✓ Formatting completed successfully")
	return nil
}

// Typecheck runs type checking on all modules
func (Quality) Typecheck(ctx context.Context) error {
	fmt.Println("Type checking all modules...")
	mg.CtxDeps(ctx, typecheckMain, typecheckSDK)
	fmt.Println("✓ Type checking completed successfully")
	return nil
}

// Modernize modernizes code patterns in all modules
func (Quality) Modernize(ctx context.Context) error {
	fmt.Println("Modernizing code patterns...")
	mg.CtxDeps(ctx, modernizeMain, modernizeSDK)
	fmt.Println("✓ Modernization completed successfully")
	return nil
}

// Start starts Docker services
func (Docker) Start() error {
	fmt.Println("Starting Docker services...")
	return sh.RunV("docker", "compose", "-f", "./cluster/docker-compose.yml", "up", "-d")
}

// Stop stops Docker services
func (Docker) Stop() error {
	fmt.Println("Stopping Docker services...")
	return sh.RunV("docker", "compose", "-f", "./cluster/docker-compose.yml", "down")
}

// Clean removes Docker volumes
func (Docker) Clean() error {
	fmt.Println("Cleaning Docker volumes...")
	return sh.RunV("docker", "compose", "-f", "./cluster/docker-compose.yml", "down", "--volumes")
}

// Reset resets Docker environment
func (Docker) Reset(ctx context.Context) error {
	mg.SerialCtxDeps(ctx, Docker.Clean, Docker.Start)
	return nil
}

// Status shows migration status
func (Database) Status() error {
	return runGoose("status")
}

// Up applies pending migrations
func (Database) Up() error {
	return runGoose("up")
}

// Down rolls back last migration
func (Database) Down() error {
	return runGoose("down")
}

// Create creates a new migration
func (Database) Create() error {
	name := os.Getenv("name")
	if name == "" {
		return fmt.Errorf("migration name required: use 'name=<name> mage database:create'")
	}
	return runGoose("create", name, "sql")
}

// Validate validates migrations
func (Database) Validate() error {
	return runGoose("validate")
}

// Reset resets database
func (Database) Reset(ctx context.Context) error {
	return runGoose("reset")
}

// CLI opens Redis CLI
func (Redis) CLI() error {
	password := getEnv("REDIS_PASSWORD", "redis_secret")
	return sh.RunV("docker", "exec", "-it", "redis", "redis-cli", "-a", password)
}

// Info shows Redis info
func (Redis) Info() error {
	password := getEnv("REDIS_PASSWORD", "redis_secret")
	return sh.RunV("docker", "exec", "redis", "redis-cli", "-a", password, "info")
}

// Monitor monitors Redis commands
func (Redis) Monitor() error {
	password := getEnv("REDIS_PASSWORD", "redis_secret")
	return sh.RunV("docker", "exec", "-it", "redis", "redis-cli", "-a", password, "monitor")
}

// Flush flushes all Redis data
func (Redis) Flush() error {
	password := getEnv("REDIS_PASSWORD", "redis_secret")
	return sh.RunV("docker", "exec", "redis", "redis-cli", "-a", password, "flushall")
}

// TestConnection tests Redis connection
func (Redis) TestConnection() error {
	fmt.Println("Testing Redis connection...")
	password := getEnv("REDIS_PASSWORD", "redis_secret")
	return sh.RunV("docker", "exec", "redis", "redis-cli", "-a", password, "ping")
}

// SdkCompozy runs the sdk/compozy integration suite
func (Integration) SdkCompozy(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "gotestsum",
		"--format", "pkgname",
		"--",
		"-race",
		"-parallel=4",
		"./sdk/compozy/...",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getIntegrationTestDirs() ([]string, error) {
	entries, err := os.ReadDir("./test/integration")
	if err != nil {
		return nil, fmt.Errorf("failed to read integration test directory: %w", err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join("./test/integration", entry.Name()))
		}
	}
	return dirs, nil
}

// Generate generates JSON schemas
func (Schema) Generate() error {
	fmt.Println("Generating schemas...")
	if err := sh.RunV("go", "run", "./pkg/schemagen", "-out=./schemas"); err != nil {
		return err
	}
	uiPath := "../compozy-ui/backend/"
	if _, err := os.Stat(uiPath); err == nil {
		fmt.Println("Copying schemas to compozy-ui...")
		return sh.RunV("cp", "-Rf", "./schemas", uiPath)
	}
	return nil
}

// Watch watches and regenerates schemas on changes
func (Schema) Watch() error {
	fmt.Println("Watching schemas for changes...")
	return sh.RunV("go", "run", "./pkg/schemagen", "-out=./schemas", "-watch")
}

// Helper functions

func checkGoVersion() error {
	fmt.Println("Checking Go version...")
	out, err := sh.Output("go", "version")
	if err != nil {
		fmt.Println("Error: Go is not available")
		fmt.Printf("Please ensure Go %s is installed via mise\n", goVersion)
		return fmt.Errorf("go not found")
	}
	parts := strings.Fields(out)
	if len(parts) < 3 {
		return fmt.Errorf("unexpected go version output: %s", out)
	}
	installedVersion := strings.TrimPrefix(parts[2], "go")
	if !isVersionCompatible(installedVersion, goVersion) {
		fmt.Printf("Warning: Go version %s found, but %s is required\n", installedVersion, goVersion)
		fmt.Printf("Please update Go to version %s with: mise use go@%s\n", goVersion, goVersion)
		return fmt.Errorf("incompatible Go version")
	}
	fmt.Printf("✓ Go version %s is compatible\n", installedVersion)
	return nil
}

func cleanGoCache() error {
	fmt.Println("Cleaning Go build cache for fresh setup...")
	_ = sh.Run("go", "clean", "-cache", "-testcache", "-modcache")
	fmt.Println("✓ Go cache cleaned")
	return nil
}

func swaggerDeps() error {
	fmt.Println("Installing Swagger dependencies...")
	if err := sh.RunV("go", "install", "github.com/swaggo/swag/cmd/swag@latest"); err != nil {
		return err
	}
	fmt.Println("Swagger dependencies installation complete.")
	return nil
}

func runTestsInParallel(ctx context.Context, dirs []string, cmdArgs ...string) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(dirs))
	for _, dir := range dirs {
		wg.Add(1)
		go func(testDir string) {
			defer wg.Done()
			fmt.Printf("Testing %s...\n", testDir)
			pkgPath := "./" + testDir + "/..."
			args := append(cmdArgs, pkgPath)
			if err := sh.RunV(args[0], args[1:]...); err != nil {
				errChan <- fmt.Errorf("%s: %w", testDir, err)
			}
		}(dir)
	}
	wg.Wait()
	close(errChan)
	for err := range errChan {
		return err
	}
	return nil
}

func testMain(ctx context.Context) error {
	start := time.Now()
	fmt.Println("Testing main module...")
	err := runTestsInParallel(ctx, mainPackages,
		"gotestsum", "--format", "pkgname", "--", "-race", "-parallel=4")
	duration := time.Since(start)
	fmt.Printf("✓ Tests completed in %s\n", duration.Round(time.Second))
	return err
}

func testSDK(ctx context.Context) error {
	fmt.Println("Testing sdk module...")
	if err := sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c",
		"cd sdk && gotestsum --format pkgname -- -race -parallel=4 ./..."); err != nil {
		return err
	}
	fmt.Println("Enforcing sdk/compozy coverage >= 85%...")
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("sdk-cover-%d.out", time.Now().UnixNano()))
	defer os.Remove(tmpFile)
	if err := sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c",
		fmt.Sprintf("cd sdk && go test -coverprofile=%s ./compozy/... > /dev/null", tmpFile)); err != nil {
		return err
	}
	coverage, err := readTotalCoverage(tmpFile)
	if err != nil {
		return err
	}
	fmt.Printf("sdk/compozy coverage: %.2f%%\n", coverage*100)
	if coverage < 0.85 {
		return fmt.Errorf("coverage %.2f%% below required 85%%", coverage*100)
	}
	return nil
}

func testBun(ctx context.Context) error {
	return sh.RunV("bun", "run", "test")
}

func testCoverageMain(ctx context.Context) error {
	start := time.Now()
	fmt.Println("Testing main module with coverage...")
	err := runTestsInParallel(ctx, mainPackages,
		"gotestsum", "--format", "pkgname", "--", "-race", "-parallel=4",
		"-coverprofile=coverage.out", "-covermode=atomic")
	duration := time.Since(start)
	fmt.Printf("✓ Tests with coverage completed in %s\n", duration.Round(time.Second))
	return err
}

func testCoverageSDK(ctx context.Context) error {
	fmt.Println("Testing sdk module with coverage...")
	return sh.RunWithV(
		map[string]string{"GO_WORK": "off"},
		"sh",
		"-c",
		"cd sdk && gotestsum --format pkgname -- -race -parallel=4 -coverprofile=coverage-sdk.out -covermode=atomic ./...",
	)
}

func testNoCacheMain(ctx context.Context) error {
	start := time.Now()
	fmt.Println("Testing main module (no cache)...")
	err := runTestsInParallel(ctx, mainPackages,
		"gotestsum", "--format", "pkgname", "--", "-race", "-count=1", "-parallel=4")
	duration := time.Since(start)
	fmt.Printf("✓ Tests (no cache) completed in %s\n", duration.Round(time.Second))
	return err
}

func testNoCacheSDK(ctx context.Context) error {
	fmt.Println("Testing sdk module (no cache)...")
	return sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c",
		"cd sdk && gotestsum --format pkgname -- -race -count=1 -parallel=4 ./...")
}

//mage:expose
func lintMain(ctx context.Context) error {
	fmt.Println("Linting main module...")
	return sh.RunV("golangci-lint", "run", "--fix", "--allow-parallel-runners", "./...")
}

//mage:expose
func lintSDK(ctx context.Context) error {
	fmt.Println("Linting sdk module...")
	return sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c",
		"cd sdk && golangci-lint run --fix --allow-parallel-runners ./...")
}

//mage:expose
func lintBun(ctx context.Context) error {
	return sh.RunV("bun", "run", "lint")
}

func checkDriverImports() error {
	fmt.Println("Running static driver import guard...")
	return sh.RunV("./scripts/check-driver-imports.sh")
}

//mage:expose
func fmtMain(ctx context.Context) error {
	fmt.Println("Formatting main module...")
	return sh.RunV("golangci-lint", "fmt")
}

//mage:expose
func fmtSDK(ctx context.Context) error {
	fmt.Println("Formatting sdk module...")
	return sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c", "cd sdk && golangci-lint fmt")
}

//mage:expose
func fmtBun(ctx context.Context) error {
	return sh.RunV("bun", "run", "format")
}

//mage:expose
func typecheckMain(ctx context.Context) error {
	fmt.Println("Type checking main module...")
	return sh.RunV("go", "vet", "./...")
}

//mage:expose
func typecheckSDK(ctx context.Context) error {
	fmt.Println("Type checking sdk module...")
	return sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c", "cd sdk && go vet ./...")
}

//mage:expose
func modernizeMain(ctx context.Context) error {
	fmt.Println("Modernizing main module...")
	return sh.RunV(
		"go",
		"run",
		"golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest",
		"-fix",
		"./...",
	)
}

//mage:expose
func modernizeSDK(ctx context.Context) error {
	fmt.Println("Modernizing sdk module...")
	return sh.RunWithV(map[string]string{"GO_WORK": "off"}, "sh", "-c",
		"cd sdk && go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix ./...")
}

func runGoose(args ...string) error {
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "compozy")
	dbString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	env := map[string]string{
		"GOOSE_DRIVER":   "postgres",
		"GOOSE_DBSTRING": dbString,
	}
	fullArgs := append([]string{"-dir", "./engine/infra/postgres/migrations"}, args...)
	return sh.RunWithV(env, "goose", fullArgs...)
}

func swaggerNeedsRebuild() (bool, error) {
	sourceFiles := []string{"main.go", "go.mod"}
	targetFile := filepath.Join(swaggerDir, "swagger.json")
	exists, err := target.Path(targetFile, sourceFiles...)
	if err != nil {
		return true, nil
	}
	return exists, nil
}

func filterSwaggerWarnings(output string) string {
	lines := strings.Split(output, "\n")
	var filtered []string
	for _, line := range lines {
		if strings.Contains(line, "warning: failed to evaluate const") ||
			strings.Contains(line, "reflect: call of reflect.Value") ||
			strings.Contains(line, "strconv.ParseUint: parsing") {
			continue
		}
		if strings.TrimSpace(line) != "" {
			filtered = append(filtered, line)
		}
	}
	return strings.Join(filtered, "\n")
}

func readTotalCoverage(path string) (float64, error) {
	cmd := exec.Command("go", "tool", "cover", "-func="+path)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	var totalLine string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "total:") {
			totalLine = line
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if totalLine == "" {
		return 0, fmt.Errorf("total coverage not found in %s", path)
	}
	fields := strings.Fields(totalLine)
	if len(fields) == 0 {
		return 0, fmt.Errorf("invalid coverage summary: %s", totalLine)
	}
	valueStr := strings.TrimSuffix(fields[len(fields)-1], "%")
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse coverage value: %w", err)
	}
	return value / 100, nil
}

func getGitCommit() string {
	out, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

func getVersion() string {
	out, err := sh.Output("git", "describe", "--tags", "--match=v*", "--always")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func isVersionCompatible(installed, required string) bool {
	installedParts := strings.Split(installed, ".")
	requiredParts := strings.Split(required, ".")
	for i := 0; i < len(requiredParts) && i < len(installedParts); i++ {
		installedNum, err1 := strconv.Atoi(installedParts[i])
		requiredNum, err2 := strconv.Atoi(requiredParts[i])
		if err1 != nil || err2 != nil {
			continue
		}
		if installedNum < requiredNum {
			return false
		}
		if installedNum > requiredNum {
			return true
		}
	}
	return true
}

// Help displays available targets
func Help() {
	fmt.Println("Compozy Mage Targets")
	fmt.Println("")
	fmt.Println("Setup & Build:")
	fmt.Println("  mage setup              - Complete setup with Go version check and dependencies")
	fmt.Println("  mage deps               - Install all required dependencies")
	fmt.Println("  mage build              - Build the compozy binary")
	fmt.Println("  mage clean              - Clean build artifacts")
	fmt.Println("")
	fmt.Println("Development:")
	fmt.Println("  mage dev                - Run in development mode with hot reload")
	fmt.Println("  mage test               - Run all tests (main + sdk + bun) in parallel")
	fmt.Println("  mage testCoverage       - Run all tests with coverage")
	fmt.Println("  mage testNoCache        - Run all tests without cache")
	fmt.Println("")
	fmt.Println("Code Quality (parallel execution):")
	fmt.Println("  mage quality:lint       - Run all linters in parallel")
	fmt.Println("  mage quality:fmt        - Format all code in parallel")
	fmt.Println("  mage quality:typecheck  - Type check all modules")
	fmt.Println("  mage quality:modernize  - Modernize code patterns")
	fmt.Println("")
	fmt.Println("Docker & Database:")
	fmt.Println("  mage docker:start       - Start Docker services")
	fmt.Println("  mage docker:stop        - Stop Docker services")
	fmt.Println("  mage docker:reset       - Reset Docker environment")
	fmt.Println("  mage database:up        - Run database migrations")
	fmt.Println("  mage database:down      - Rollback last migration")
	fmt.Println("")
	fmt.Println("Other:")
	fmt.Println("  mage all                - Run all checks (tests, lint, format)")
	fmt.Println("  mage swagger            - Generate Swagger documentation")
	fmt.Println("  mage schema:generate    - Generate JSON schemas")
	fmt.Println("  mage -l                 - List all available targets")
	fmt.Println("")
	fmt.Println("Requirements:")
	fmt.Printf("  Go %s or later (via mise)\n", goVersion)
	fmt.Println("  Bun (see https://bun.sh)")
	fmt.Println("  Docker & Docker Compose")
	fmt.Println("")
	fmt.Println("Performance:")
	fmt.Println("  Tests run ~2-3x faster (parallel execution)")
	fmt.Println("  Linting runs ~1.5-2x faster (parallel execution)")
	fmt.Println("  Smart caching skips unnecessary rebuilds")
	fmt.Println("")
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
