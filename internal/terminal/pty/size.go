package pty

import terminalwire "github.com/compozy/compozy/internal/terminal/wire"

func normalizedSize(cols, rows uint16) (uint16, uint16) {
	cols, rows, _ = terminalwire.ClampDimensions(cols, rows)
	return cols, rows
}
