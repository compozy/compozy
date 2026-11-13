package contract

// WindowManagerMigrationRequest acknowledges a notice or retries preserved pre-v6 bytes.
type WindowManagerMigrationRequest struct {
	Action  string `json:"action"`
	Replace bool   `json:"replace"`
}
