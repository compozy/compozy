package mcpproxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultHTTPTimeout         = 30 * time.Second
	defaultHealthCheckInterval = 30 * time.Second
	defaultMaxReconnects       = 5
	defaultReconnectDelay      = 5 * time.Second
)

// TransportType represents the transport type for MCP communication
type TransportType string

const (
	TransportStdio          TransportType = "stdio"
	TransportSSE            TransportType = "sse"
	TransportStreamableHTTP TransportType = "streamable-http"
)

// String returns the string representation of the transport type
func (t TransportType) String() string {
	return string(t)
}

// IsValid checks if the transport type is valid
func (t TransportType) IsValid() bool {
	switch t {
	case TransportStdio, TransportSSE, TransportStreamableHTTP:
		return true
	default:
		return false
	}
}

// ConnectionStatus represents the current status of an MCP connection
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusError        ConnectionStatus = "error"
)

// MCPDefinition represents a complete MCP server definition
type MCPDefinition struct {
	// Core identification
	Name        string        `json:"name"                  validate:"required,min=1"`
	Description string        `json:"description,omitempty"`
	Transport   TransportType `json:"transport"             validate:"required"`

	// Stdio transport configuration
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// HTTP-based transport configuration (SSE and streamable-http)
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"`

	// Behavior configuration
	AutoReconnect       bool          `json:"autoReconnect,omitempty"`
	MaxReconnects       int           `json:"maxReconnects,omitempty"`
	ReconnectDelay      time.Duration `json:"reconnectDelay,omitempty"`
	HealthCheckEnabled  bool          `json:"healthCheckEnabled,omitempty"`
	HealthCheckInterval time.Duration `json:"healthCheckInterval,omitempty"`
	LogEnabled          bool          `json:"logEnabled,omitempty"`

	// Tool filtering
	ToolFilter *ToolFilter `json:"toolFilter,omitempty"`

	// Metadata
	Tags      map[string]string `json:"tags,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// ToolFilter defines tool filtering configuration
type ToolFilter struct {
	Mode ToolFilterMode `json:"mode"           validate:"required"`
	List []string       `json:"list,omitempty"`
}

// ToolFilterMode represents the tool filtering mode
type ToolFilterMode string

const (
	ToolFilterAllow ToolFilterMode = "allow"
	ToolFilterBlock ToolFilterMode = "block"
)

// MCPStatus represents the runtime status of an MCP connection
type MCPStatus struct {
	Name              string           `json:"name"`
	Status            ConnectionStatus `json:"status"`
	LastConnected     *time.Time       `json:"lastConnected,omitempty"`
	LastError         string           `json:"lastError,omitempty"`
	LastErrorTime     *time.Time       `json:"lastErrorTime,omitempty"`
	ReconnectAttempts int              `json:"reconnectAttempts"`
	UpTime            time.Duration    `json:"upTime"`
	TotalRequests     int64            `json:"totalRequests"`
	TotalErrors       int64            `json:"totalErrors"`
	AvgResponseTime   time.Duration    `json:"avgResponseTime"`

	// Thread safety
	mu sync.RWMutex `json:"-"` // Protects all fields above
}

// Validate validates the MCP definition

func (m *MCPDefinition) Validate() error {
	if err := m.validateBasicFields(); err != nil {
		return err
	}
	if err := m.validateTransport(); err != nil {
		return err
	}
	if err := m.validateOptionalFields(); err != nil {
		return err
	}
	return nil
}

// validateBasicFields validates required basic fields
func (m *MCPDefinition) validateBasicFields() error {
	if m.Name == "" {
		return errors.New("name is required")
	}
	if !m.Transport.IsValid() {
		return fmt.Errorf("invalid transport type: %s", m.Transport)
	}
	return nil
}

// validateTransport validates transport-specific fields
func (m *MCPDefinition) validateTransport() error {
	switch m.Transport {
	case TransportStdio:
		if m.Command == "" {
			return errors.New("command is required for stdio transport")
		}
	case TransportSSE, TransportStreamableHTTP:
		if m.URL == "" {
			return errors.New("url is required for HTTP-based transports")
		}
	}
	return nil
}

// validateOptionalFields validates optional configuration fields
func (m *MCPDefinition) validateOptionalFields() error {
	if m.ToolFilter != nil {
		if err := m.ToolFilter.Validate(); err != nil {
			return fmt.Errorf("tool filter validation failed: %w", err)
		}
	}
	if m.Timeout < 0 {
		return errors.New("timeout cannot be negative")
	}
	if m.MaxReconnects < 0 {
		return errors.New("maxReconnects cannot be negative")
	}
	if m.ReconnectDelay < 0 {
		return errors.New("reconnectDelay cannot be negative")
	}
	if m.HealthCheckInterval < 0 {
		return errors.New("healthCheckInterval cannot be negative")
	}
	return nil
}

// Note: per-MCP IP allowlist validation has been removed.

// Validate validates the tool filter configuration
func (tf *ToolFilter) Validate() error {
	if tf.Mode != ToolFilterAllow && tf.Mode != ToolFilterBlock {
		return fmt.Errorf("invalid tool filter mode: %s", tf.Mode)
	}
	if len(tf.List) == 0 {
		return errors.New("tool filter list cannot be empty")
	}
	return nil
}

// SetDefaults sets default values for optional fields
func (m *MCPDefinition) SetDefaults() {
	m.setTimestampDefaults()
	m.setTimeoutDefaults()
	m.setReconnectDefaults()
	m.setMapDefaults()
	m.setSliceDefaults()
}

// setTimestampDefaults sets default timestamps
func (m *MCPDefinition) setTimestampDefaults() {
	now := time.Now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
}

// setTimeoutDefaults sets default timeout values
func (m *MCPDefinition) setTimeoutDefaults() {
	if m.Timeout == 0 && (m.Transport == TransportSSE || m.Transport == TransportStreamableHTTP) {
		m.Timeout = defaultHTTPTimeout
	}
	if m.HealthCheckInterval == 0 && m.HealthCheckEnabled {
		m.HealthCheckInterval = defaultHealthCheckInterval
	}
}

// setReconnectDefaults sets default reconnection values
func (m *MCPDefinition) setReconnectDefaults() {
	if m.MaxReconnects == 0 && m.AutoReconnect {
		m.MaxReconnects = defaultMaxReconnects
	}
	if m.ReconnectDelay == 0 && m.AutoReconnect {
		m.ReconnectDelay = defaultReconnectDelay
	}
}

// setMapDefaults sets default map values
func (m *MCPDefinition) setMapDefaults() {
	if m.Env == nil && m.Transport == TransportStdio {
		m.Env = make(map[string]string)
	}
	if m.Headers == nil && (m.Transport == TransportSSE || m.Transport == TransportStreamableHTTP) {
		m.Headers = make(map[string]string)
	}
	if m.Tags == nil {
		m.Tags = make(map[string]string)
	}
}

// setSliceDefaults sets default slice values
func (m *MCPDefinition) setSliceDefaults() {
	if m.Args == nil {
		m.Args = make([]string, 0)
	}
	if m.ToolFilter != nil && m.ToolFilter.List == nil {
		m.ToolFilter.List = make([]string, 0)
	}
}

// Clone creates a deep copy of the MCP definition
func (m *MCPDefinition) Clone() (*MCPDefinition, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("clone marshal: %w", err)
	}
	var clone MCPDefinition
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("clone unmarshal: %w", err)
	}
	return &clone, nil
}

// GetNamespace returns the Redis namespace for this definition
func (m *MCPDefinition) GetNamespace() string {
	return fmt.Sprintf("mcp_proxy:%s", sanitizeRedisKeyComponent(m.Name))
}

func sanitizeRedisKeyComponent(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ':' || r == '_' || r == '-':
			b.WriteRune(r)
		case r == ' ':
			b.WriteRune('_')
		default:
			b.WriteRune('-')
		}
	}
	return b.String()
}

// ToJSON converts the definition to JSON
func (m *MCPDefinition) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// FromJSON creates a definition from JSON
func FromJSON(data []byte) (*MCPDefinition, error) {
	var def MCPDefinition
	if err := json.Unmarshal(data, &def); err != nil {
		return nil, err
	}
	if err := def.Validate(); err != nil {
		return nil, err
	}
	def.SetDefaults()
	return &def, nil
}

// NewMCPStatus creates a new status object for the given definition
func NewMCPStatus(name string) *MCPStatus {
	return &MCPStatus{
		Name:              name,
		Status:            StatusDisconnected,
		ReconnectAttempts: 0,
		TotalRequests:     0,
		TotalErrors:       0,
	}
}

// UpdateStatus updates the connection status (thread-safe)
func (s *MCPStatus) UpdateStatus(status ConnectionStatus, errorMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = status
	now := time.Now()
	switch status {
	case StatusConnected:
		s.LastConnected = &now
		s.LastError = ""
		s.LastErrorTime = nil
		s.ReconnectAttempts = 0
	case StatusError:
		s.LastError = errorMsg
		s.LastErrorTime = &now
		s.TotalErrors++
	case StatusConnecting:
		s.ReconnectAttempts++
	}
}

// RecordRequest records a successful request (thread-safe)
func (s *MCPStatus) RecordRequest(responseTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalRequests++
	// Calculate rolling average response time
	if s.TotalRequests == 1 {
		s.AvgResponseTime = responseTime
	} else {
		// Simple exponential moving average
		s.AvgResponseTime = time.Duration(float64(s.AvgResponseTime)*0.9 + float64(responseTime)*0.1)
	}
}

// CalculateUpTime calculates the uptime since last connected (thread-safe)
func (s *MCPStatus) CalculateUpTime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.LastConnected == nil || s.Status != StatusConnected {
		return 0
	}
	return time.Since(*s.LastConnected)
}

// IncrementErrors safely increments the error count
func (s *MCPStatus) IncrementErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalErrors++
}

// SafeCopy returns a thread-safe copy of the status with calculated uptime
func (s *MCPStatus) SafeCopy() *MCPStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Create a copy of all fields
	statusCopy := MCPStatus{
		Name:              s.Name,
		Status:            s.Status,
		LastError:         s.LastError,
		ReconnectAttempts: s.ReconnectAttempts,
		TotalRequests:     s.TotalRequests,
		TotalErrors:       s.TotalErrors,
		AvgResponseTime:   s.AvgResponseTime,
	}
	// Copy pointer fields safely
	if s.LastConnected != nil {
		connectedTime := *s.LastConnected
		statusCopy.LastConnected = &connectedTime
	}
	if s.LastErrorTime != nil {
		errorTime := *s.LastErrorTime
		statusCopy.LastErrorTime = &errorTime
	}
	// Calculate uptime for the copy
	if statusCopy.LastConnected != nil && statusCopy.Status == StatusConnected {
		statusCopy.UpTime = time.Since(*statusCopy.LastConnected)
	}
	return &statusCopy
}
