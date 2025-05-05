package project

import (
	"github.com/compozy/compozy/parser/package_ref"
)

// ProjectName represents a project name
type ProjectName string

// ProjectVersion represents a project version
type ProjectVersion string

// ProjectDescription represents a project description
type ProjectDescription string

// LogLevel represents a log level
type LogLevel string

// EnvFilePath represents an environment file path
type EnvFilePath string

// Dependencies represents project dependencies
type Dependencies []*package_ref.PackageRef

// Environment represents an environment type
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentStaging     Environment = "staging"
)
