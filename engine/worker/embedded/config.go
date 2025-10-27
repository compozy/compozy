package embedded

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultDatabaseFile  = ":memory:"
	defaultFrontendPort  = 7233
	defaultBindIP        = "127.0.0.1"
	defaultNamespace     = "default"
	defaultClusterName   = "compozy-standalone"
	defaultEnableUI      = true
	defaultUIPort        = 8233
	defaultLogLevel      = "warn"
	defaultStartTimeout  = 30 * time.Second
	maxServicePortOffset = 3
	maxPort              = 65535
)

var allowedLogLevels = map[string]struct{}{
	"debug": {},
	"info":  {},
	"warn":  {},
	"error": {},
}

// Config holds embedded Temporal server configuration.
type Config struct {
	// DatabaseFile specifies SQLite database location.
	// Use ":memory:" for ephemeral in-memory storage.
	// Use file path for persistent storage across restarts.
	DatabaseFile string

	// FrontendPort is the gRPC port for the frontend service.
	FrontendPort int

	// BindIP is the IP address to bind all services to.
	BindIP string

	// Namespace is the default namespace to create on startup.
	Namespace string

	// ClusterName is the Temporal cluster name.
	ClusterName string

	// EnableUI enables the Temporal Web UI server.
	// Set to true to enable the UI server on the specified UIPort.
	EnableUI bool

	// RequireUI enforces UI availability; Start returns an error if the UI fails to launch.
	RequireUI bool

	// UIPort is the HTTP port for the Web UI.
	UIPort int

	// LogLevel controls server logging verbosity.
	LogLevel string

	// StartTimeout is the maximum time to wait for server startup.
	StartTimeout time.Duration
}

func applyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}
	if cfg.DatabaseFile == "" {
		cfg.DatabaseFile = defaultDatabaseFile
	}
	if cfg.FrontendPort == 0 {
		cfg.FrontendPort = defaultFrontendPort
	}
	if cfg.BindIP == "" {
		cfg.BindIP = defaultBindIP
	}
	if cfg.Namespace == "" {
		cfg.Namespace = defaultNamespace
	}
	if cfg.ClusterName == "" {
		cfg.ClusterName = defaultClusterName
	}
	if cfg.UIPort == 0 {
		cfg.UIPort = defaultUIPort
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaultLogLevel
	}
	if cfg.StartTimeout == 0 {
		cfg.StartTimeout = defaultStartTimeout
	}
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if err := validateDatabaseFile(cfg.DatabaseFile); err != nil {
		return err
	}
	if err := validateFrontendPort(cfg.FrontendPort); err != nil {
		return err
	}
	if err := validatePort("ui_port", cfg.UIPort, cfg.EnableUI); err != nil {
		return err
	}
	if cfg.RequireUI && !cfg.EnableUI {
		return errors.New("require_ui cannot be set when enable_ui is false")
	}
	if err := validateBindIP(cfg.BindIP); err != nil {
		return err
	}
	if cfg.Namespace == "" {
		return errors.New("namespace is required")
	}
	if cfg.ClusterName == "" {
		return errors.New("cluster name is required")
	}
	if err := validateLogLevel(cfg.LogLevel); err != nil {
		return err
	}
	if cfg.StartTimeout <= 0 {
		return errors.New("start timeout must be positive")
	}
	return nil
}

func validateDatabaseFile(path string) error {
	if path == "" {
		return errors.New("database file is required")
	}
	if path == ":memory:" {
		return nil
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve database file: %w", err)
	}
	dir := filepath.Dir(abs)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("database directory %q not accessible: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("database directory %q is not a directory", dir)
	}
	return nil
}

func validateFrontendPort(port int) error {
	if err := validatePort("frontend_port", port, true); err != nil {
		return err
	}
	if port+maxServicePortOffset > maxPort {
		return fmt.Errorf("frontend port %d reserves out-of-range service port", port)
	}
	return nil
}

func validatePort(field string, port int, required bool) error {
	if !required && port == 0 {
		return nil
	}
	if port <= 0 || port > maxPort {
		return fmt.Errorf("%s must be between 1 and %d", field, maxPort)
	}
	return nil
}

func validateBindIP(ip string) error {
	if ip == "" {
		return errors.New("bind IP is required")
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		return fmt.Errorf("invalid bind IP %q", ip)
	}
	return nil
}

func validateLogLevel(level string) error {
	if level == "" {
		return errors.New("log level is required")
	}
	if _, ok := allowedLogLevels[level]; !ok {
		return fmt.Errorf("invalid log level %q", level)
	}
	return nil
}
