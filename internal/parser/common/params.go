package common

// WithParams represents parameters for a component
type WithParams map[string]interface{}

// EnvMap represents environment variables for a component
type EnvMap map[string]string

// Merge merges another environment map into this one
func (e EnvMap) Merge(other EnvMap) {
	for k, v := range other {
		e[k] = v
	}
}

// LoadFromFile loads environment variables from a file
func (e EnvMap) LoadFromFile(path string) error {
	// TODO: Implement loading from .env file
	return nil
}
