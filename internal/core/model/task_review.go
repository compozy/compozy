package model

import "time"

type IssueEntry struct {
	Name     string
	AbsPath  string
	Content  string
	CodeFile string
}

type ReviewContext struct {
	Status      string
	File        string
	Line        int
	Severity    string
	Author      string
	ProviderRef string
}

type RoundMeta struct {
	Provider   string
	PR         string
	Round      int
	CreatedAt  time.Time
	Total      int
	Resolved   int
	Unresolved int
}

type TaskMeta struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Total     int
	Completed int
	Pending   int
}

type TaskEntry struct {
	Content      string
	Status       string
	Title        string
	TaskType     string
	Complexity   string
	Dependencies []string
}

type TaskFileMeta struct {
	Status       string   `yaml:"status"`
	Title        string   `yaml:"title"`
	TaskType     string   `yaml:"type"`
	Complexity   string   `yaml:"complexity,omitempty"`
	Dependencies []string `yaml:"dependencies"`
}

type ReviewFileMeta struct {
	Status      string `yaml:"status"`
	File        string `yaml:"file,omitempty"`
	Line        int    `yaml:"line,omitempty"`
	Severity    string `yaml:"severity,omitempty"`
	Author      string `yaml:"author,omitempty"`
	ProviderRef string `yaml:"provider_ref,omitempty"`
}
