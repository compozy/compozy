package exec

import (
	"regexp"
	"time"
)

const toolID = "cp__exec"

var defaultArgPattern = regexp.MustCompile(`^[\w\-./@=+:,]+$`)

type commandPolicy struct {
	Path            string
	Description     string
	Timeout         time.Duration
	MaxArgs         int
	AllowAdditional bool
	ArgRules        []argumentRule
}

type argumentRule struct {
	Index    int
	Optional bool
	Pattern  *regexp.Regexp
	Enum     map[string]struct{}
}

type toolConfig struct {
	Timeout   time.Duration
	MaxStdout int64
	MaxStderr int64
	Commands  map[string]*commandPolicy
}

type Args struct {
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	WorkingDir  string            `json:"working_dir"`
	TimeoutMs   int               `json:"timeout_ms"`
	Environment map[string]string `json:"env"`
}
