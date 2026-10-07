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
	YieldMS int                     `json:"yield_ms,omitzero"`
	Visible bool                    `json:"visible,omitzero"`
	Output  terminalpkg.OutputShape `json:"output,omitzero"`
}

type terminalOpenInput struct {
	Cwd   string `json:"cwd,omitempty"`
	Shell string `json:"shell,omitempty"`
	Cols  uint16 `json:"cols,omitzero"`
	Rows  uint16 `json:"rows,omitzero"`
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
	MaxBytes   int    `json:"max_bytes,omitzero"`
	SinceSeq   string `json:"since_seq,omitempty"`
	From       int    `json:"from,omitzero"`
	To         int    `json:"to,omitzero"`
	Grep       string `json:"grep,omitempty"`
}

type terminalWaitInput struct {
	TerminalID string `json:"terminal_id"`
	Until      string `json:"until"`
	Pattern    string `json:"pattern,omitempty"`
	TimeoutMS  int    `json:"timeout_ms,omitzero"`
}

type terminalSignalInput struct {
	TerminalID string `json:"terminal_id"`
	Signal     string `json:"signal"`
}

type terminalInputRequestInput struct {
	TerminalID    string `json:"terminal_id"`
	Reason        string `json:"reason"`
	PromptExcerpt string `json:"prompt_excerpt"`
	Redact        bool   `json:"redact,omitzero"`
}

type terminalToolInfo = contract.TerminalInfoPayload
