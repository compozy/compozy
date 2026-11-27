package contract

import "github.com/compozy/compozy/internal/remoteproto"

const (
	ComputerAddKindSSH = "ssh"
	ComputerSetupsPath = ComputersPath + "/setups"
)

type ComputerSSHHost struct {
	Alias    string `json:"alias"`
	Hostname string `json:"hostname"`
	User     string `json:"user"`
	Port     int    `json:"port"`
	Source   string `json:"source"`
}

type ComputerSSHHostsResponse struct {
	Hosts []ComputerSSHHost `json:"hosts"`
}

type ComputerSetupStarted struct {
	SetupID    string                  `json:"setup_id"`
	ComputerID *remoteproto.ComputerID `json:"computer_id"`
}

type ComputerUpdateStarted struct {
	SetupID string `json:"setup_id"`
}

type ComputerSetupStatus struct {
	SetupID    string                  `json:"setup_id"`
	Step       string                  `json:"step"`
	State      string                  `json:"state"`
	Prompt     *ComputerSetupPrompt    `json:"prompt"`
	PromptKind string                  `json:"prompt_kind,omitempty"`
	Error      *ErrorPayload           `json:"error"`
	ComputerID *remoteproto.ComputerID `json:"computer_id"`
}

type ComputerSetupPrompt struct {
	Kind        string `json:"kind"`
	Host        string `json:"host,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type ComputerSetupAnswer struct {
	Kind   string `json:"kind"`
	Accept *bool  `json:"accept,omitempty"`
	Value  string `json:"value,omitempty"`
}

type ComputerSetupCanceled struct {
	SetupID string `json:"setup_id"`
	State   string `json:"state"`
}
