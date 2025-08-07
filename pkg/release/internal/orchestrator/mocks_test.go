package orchestrator

import (
	"context"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/stretchr/testify/mock"
)

// Mock for GitExtendedRepository - implements ALL methods from GitExtendedRepository interface
type mockGitExtendedRepository struct{ mock.Mock }

// GitRepository methods
func (m *mockGitExtendedRepository) LatestTag(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
func (m *mockGitExtendedRepository) CommitsSinceTag(ctx context.Context, tag string) (int, error) {
	args := m.Called(ctx, tag)
	return args.Int(0), args.Error(1)
}
func (m *mockGitExtendedRepository) TagExists(ctx context.Context, tag string) (bool, error) {
	args := m.Called(ctx, tag)
	return args.Bool(0), args.Error(1)
}
func (m *mockGitExtendedRepository) CreateBranch(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) CreateTag(ctx context.Context, tag, msg string) error {
	args := m.Called(ctx, tag, msg)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) PushTag(ctx context.Context, tag string) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

// GitExtendedRepository specific methods
func (m *mockGitExtendedRepository) CheckoutBranch(ctx context.Context, branch string) error {
	args := m.Called(ctx, branch)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) ConfigureUser(ctx context.Context, name, email string) error {
	args := m.Called(ctx, name, email)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) AddFiles(ctx context.Context, pattern string) error {
	args := m.Called(ctx, pattern)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) Commit(ctx context.Context, message string) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
func (m *mockGitExtendedRepository) PushBranch(ctx context.Context, branch string) error {
	args := m.Called(ctx, branch)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) DeleteBranch(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) DeleteRemoteBranch(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) RestoreFile(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) ResetHard(ctx context.Context, ref string) error {
	args := m.Called(ctx, ref)
	return args.Error(0)
}
func (m *mockGitExtendedRepository) GetHeadCommit(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
func (m *mockGitExtendedRepository) ListLocalBranches(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if branches := args.Get(0); branches != nil {
		return branches.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockGitExtendedRepository) ListRemoteBranches(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if branches := args.Get(0); branches != nil {
		return branches.([]string), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockGitExtendedRepository) GetFileStatus(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

// Mock for GithubExtendedRepository
type mockGithubExtendedRepository struct{ mock.Mock }

func (m *mockGithubExtendedRepository) CreatePullRequest(
	ctx context.Context,
	title, body, head, base string,
) (int, error) {
	args := m.Called(ctx, title, body, head, base)
	return args.Int(0), args.Error(1)
}

func (m *mockGithubExtendedRepository) CreateOrUpdatePR(
	ctx context.Context,
	head, base, title, body string,
	labels []string,
) error {
	args := m.Called(ctx, head, base, title, body, labels)
	return args.Error(0)
}
func (m *mockGithubExtendedRepository) AddComment(ctx context.Context, prNumber int, body string) error {
	args := m.Called(ctx, prNumber, body)
	return args.Error(0)
}
func (m *mockGithubExtendedRepository) ClosePR(ctx context.Context, prNumber int) error {
	args := m.Called(ctx, prNumber)
	return args.Error(0)
}
func (m *mockGithubExtendedRepository) GetPRStatus(ctx context.Context, prNumber int) (string, error) {
	args := m.Called(ctx, prNumber)
	return args.String(0), args.Error(1)
}

// Mock for CliffService
type mockCliffService struct{ mock.Mock }

func (m *mockCliffService) CalculateNextVersion(ctx context.Context, latestTag string) (*domain.Version, error) {
	args := m.Called(ctx, latestTag)
	if v := args.Get(0); v != nil {
		return v.(*domain.Version), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockCliffService) GenerateChangelog(ctx context.Context, version, mode string) (string, error) {
	args := m.Called(ctx, version, mode)
	return args.String(0), args.Error(1)
}

// Mock for NpmService
type mockNpmService struct{ mock.Mock }

func (m *mockNpmService) Publish(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

// Mock for GoReleaserService
type mockGoReleaserService struct{ mock.Mock }

func (m *mockGoReleaserService) Run(ctx context.Context, args ...string) error {
	callArgs := []any{ctx}
	for _, a := range args {
		callArgs = append(callArgs, a)
	}
	result := m.Called(callArgs...)
	return result.Error(0)
}

// Mock for StateRepository
type mockStateRepository struct{ mock.Mock }

func (m *mockStateRepository) Save(ctx context.Context, state *domain.RollbackState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *mockStateRepository) Load(ctx context.Context, sessionID string) (*domain.RollbackState, error) {
	args := m.Called(ctx, sessionID)
	if state := args.Get(0); state != nil {
		return state.(*domain.RollbackState), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockStateRepository) LoadLatest(ctx context.Context) (*domain.RollbackState, error) {
	args := m.Called(ctx)
	if state := args.Get(0); state != nil {
		return state.(*domain.RollbackState), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockStateRepository) Delete(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *mockStateRepository) Exists(ctx context.Context, sessionID string) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}
