package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalExecInput struct {
	Command string                  `json:"command"`
	Args    []string                `json:"args,omitempty"`
	Cwd     string                  `json:"cwd,omitempty"`
	Env     map[string]string       `json:"env,omitempty"`
	YieldMS int                     `json:"yield_ms,omitempty"`
	Visible bool                    `json:"visible,omitempty"`
	Output  terminalpkg.OutputShape `json:"output,omitempty"`
}

type terminalOpenInput struct {
	Cwd   string `json:"cwd,omitempty"`
	Shell string `json:"shell,omitempty"`
	Cols  uint16 `json:"cols,omitempty"`
	Rows  uint16 `json:"rows,omitempty"`
	Title string `json:"title"`
}

type terminalIDInput struct {
	TerminalID string `json:"terminal_id"`
}

type terminalWriteInput struct {
	TerminalID string `json:"terminal_id"`
	Data       string `json:"data,omitempty"`
}

type terminalReadInput struct {
	TerminalID string `json:"terminal_id"`
	View       string `json:"view"`
	MaxBytes   int    `json:"max_bytes,omitempty"`
	SinceSeq   uint64 `json:"since_seq,omitempty"`
	From       int    `json:"from,omitempty"`
	To         int    `json:"to,omitempty"`
	Grep       string `json:"grep,omitempty"`
}

type terminalWaitInput struct {
	TerminalID string `json:"terminal_id"`
	Until      string `json:"until"`
	Pattern    string `json:"pattern,omitempty"`
	TimeoutMS  int    `json:"timeout_ms,omitempty"`
}

type terminalSignalInput struct {
	TerminalID string `json:"terminal_id"`
	Signal     string `json:"signal"`
}

type terminalInputRequestInput struct {
	TerminalID    string `json:"terminal_id"`
	Reason        string `json:"reason"`
	PromptExcerpt string `json:"prompt_excerpt"`
	Redact        bool   `json:"redact,omitempty"`
}

type terminalYieldInput struct {
	TerminalID string `json:"terminal_id"`
	Reason     string `json:"reason"`
}

type terminalToolInfo = contract.TerminalInfoPayload
