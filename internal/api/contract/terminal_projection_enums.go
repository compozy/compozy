package contract

func TerminalModeValues() []string {
	return []string{"pty", "pipe"}
}

func TerminalLeaseStateValues() []string {
	return []string{"human_owned", "agent_owned", "available"}
}

func TerminalActorKindValues() []string {
	return []string{"human", "agent", "system"}
}

func TerminalSignalValues() []string {
	return []string{"INT", "TERM", "KILL", "HUP"}
}
