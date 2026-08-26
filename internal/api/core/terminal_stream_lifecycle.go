package core

type terminalStreamLifecycle = streamLifecycle

func newTerminalStreamLifecycle() *terminalStreamLifecycle {
	return newStreamLifecycle("terminal")
}
