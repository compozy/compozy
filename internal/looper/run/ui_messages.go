package run

type terminalOutputMsg struct {
	Index int
	Data  []byte
}

type terminalReadyMsg struct {
	Index int
}

type jobDoneSignalMsg struct {
	JobID string
}

type composerSendMsg struct {
	Index   int
	Message string
}
