package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	GithubToken string `mapstructure:"github_token"`
	GithubOwner string `mapstructure:"github_owner"`
	GithubRepo  string `mapstructure:"github_repo"`
	ToolsDir    string `mapstructure:"tools_dir"`
	NpmToken    string `mapstructure:"npm_token"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		GithubOwner: "compozy",
		GithubRepo:  "compozy",
		ToolsDir:    "tools",
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// GitHub token is optional - only validate if provided
	if c.GithubToken != "" {
		if err := ValidateGitHubToken(c.GithubToken); err != nil {
			return fmt.Errorf("invalid github_token: %w", err)
		}
	}
	// Validate GitHub owner and repo
	if err := ValidateGitHubOwnerRepo(c.GithubOwner, c.GithubRepo); err != nil {
		return fmt.Errorf("invalid github configuration: %w", err)
	}
	// Validate tools directory
	if c.ToolsDir == "" {
		return fmt.Errorf("tools_dir cannot be empty")
	}
	// Check for path traversal in tools directory
	if strings.Contains(c.ToolsDir, "..") {
		return fmt.Errorf("tools_dir contains invalid path traversal")
	}
	return nil
}

// ValidateForGitHubOperations validates that GitHub token is present for operations that require it
func (c *Config) ValidateForGitHubOperations() error {
	if c.GithubToken == "" {
		return fmt.Errorf("github_token is required for GitHub operations")
	}
	return c.Validate()
}

// ValidateGitHubToken validates GitHub token format (exported for reuse)
func ValidateGitHubToken(token string) error {
	token = strings.TrimSpace(token)
	if len(token) < 40 {
		return fmt.Errorf("token too short: expected at least 40 characters")
	}
	// Validate token format patterns
	classicPAT := regexp.MustCompile(`^[a-fA-F0-9]{40}$`)
	fineGrainedPAT := regexp.MustCompile(`^github_pat_[a-zA-Z0-9_]{82}$`)
	appToken := regexp.MustCompile(`^ghs_[a-zA-Z0-9]{36}$`)
	oauthToken := regexp.MustCompile(`^gho_[a-zA-Z0-9]{36}$`)
	if !classicPAT.MatchString(token) &&
		!fineGrainedPAT.MatchString(token) &&
		!appToken.MatchString(token) &&
		!oauthToken.MatchString(token) {
		return fmt.Errorf("invalid token format")
	}
	return nil
}

// ValidateGitHubOwnerRepo validates GitHub owner and repository names (exported for reuse)
func ValidateGitHubOwnerRepo(owner, repo string) error {
	if owner == "" {
		return fmt.Errorf("owner cannot be empty")
	}
	if repo == "" {
		return fmt.Errorf("repository cannot be empty")
	}
	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-_.]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)
	if !validName.MatchString(owner) {
		return fmt.Errorf("invalid owner format: %s", owner)
	}
	if len(owner) > 39 {
		return fmt.Errorf("owner too long: maximum 39 characters")
	}
	if !validName.MatchString(repo) {
		return fmt.Errorf("invalid repository format: %s", repo)
	}
	if len(repo) > 100 {
		return fmt.Errorf("repository too long: maximum 100 characters")
	}
	return nil
}

func LoadConfig() (*Config, error) {
	viper.SetConfigName(".compozy-release")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Configure environment variables
	viper.SetEnvPrefix("COMPOZY_RELEASE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// Explicitly bind environment variables
	// BindEnv allows multiple env vars - it will check them in order
	if err := viper.BindEnv("github_token", "GITHUB_TOKEN", "COMPOZY_RELEASE_GITHUB_TOKEN"); err != nil {
		return nil, fmt.Errorf("failed to bind github_token env: %w", err)
	}
	if err := viper.BindEnv("github_owner", "GITHUB_OWNER", "COMPOZY_RELEASE_GITHUB_OWNER"); err != nil {
		return nil, fmt.Errorf("failed to bind github_owner env: %w", err)
	}
	if err := viper.BindEnv("github_repo", "GITHUB_REPO", "COMPOZY_RELEASE_GITHUB_REPO"); err != nil {
		return nil, fmt.Errorf("failed to bind github_repo env: %w", err)
	}
	if err := viper.BindEnv("tools_dir", "TOOLS_DIR", "COMPOZY_RELEASE_TOOLS_DIR"); err != nil {
		return nil, fmt.Errorf("failed to bind tools_dir env: %w", err)
	}
	if err := viper.BindEnv("npm_token", "NPM_TOKEN", "COMPOZY_RELEASE_NPM_TOKEN"); err != nil {
		return nil, fmt.Errorf("failed to bind npm_token env: %w", err)
	}
	// Set defaults
	defaults := DefaultConfig()
	viper.SetDefault("github_owner", defaults.GithubOwner)
	viper.SetDefault("github_repo", defaults.GithubRepo)
	viper.SetDefault("tools_dir", defaults.ToolsDir)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}
	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return &config, nil
}
