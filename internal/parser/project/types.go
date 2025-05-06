package project

import (
	"github.com/compozy/compozy/internal/parser/package_ref"
)

type ProjectName string
type ProjectVersion string
type ProjectDescription string
type LogLevel string
type EnvFilePath string
type Dependencies []*package_ref.PackageRef
type Environment string

const (
	EnvironmentDevelopment Environment = "development"
	EnvironmentProduction  Environment = "production"
	EnvironmentStaging     Environment = "staging"
)
