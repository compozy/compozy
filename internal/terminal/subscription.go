package terminal

import (
	"sync"

	terminalwire "github.com/compozy/compozy/internal/terminal/wire"
)

type subscription struct {
	session    *session
	id         uint64
	mode       string
	actor      Actor
	queue      *terminalwire.Queue
	cols       uint16
	rows       uint16
	removeOnce sync.Once
	finishOnce sync.Once
}

var _ Subscription = (*subscription)(nil)

type attachedFramePayload struct {
	Seq       string `json:"seq"`
	Truncated bool   `json:"truncated"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
	Mode      Mode   `json:"mode"`
	Preamble  string `json:"preamble,omitempty"`
}

type exitFramePayload struct {
	Cause    string  `json:"cause"`
	ExitCode *int    `json:"exit_code"`
	Signal   *string `json:"signal"`
	Seq      string  `json:"seq"`
}

type presenceFramePayload struct {
	Viewers int `json:"viewers"`
}

type redactedInputFramePayload struct {
	Seq        string `json:"seq"`
	Characters int    `json:"characters"`
}
