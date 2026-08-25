package core

import (
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type terminalControllerPayload struct {
	Kind terminalpkg.ActorKind `json:"kind"`
	ID   string                `json:"id"`
}

type terminalRunPayload struct {
	SessionID string `json:"session_id"`
	RunID     string `json:"run_id"`
}

type terminalExitPayload struct {
	Cause  string    `json:"cause"`
	Code   *int      `json:"code,omitempty"`
	Signal *string   `json:"signal,omitempty"`
	At     time.Time `json:"at"`
}

type terminalInfoPayload struct {
	ID           terminalpkg.ID             `json:"id"`
	WorkspaceID  string                     `json:"workspace_id"`
	ProfileID    string                     `json:"profile_id"`
	ProfileName  string                     `json:"profile_name"`
	Title        string                     `json:"title"`
	Shell        string                     `json:"shell"`
	Cwd          string                     `json:"cwd"`
	Mode         terminalpkg.Mode           `json:"mode"`
	State        string                     `json:"state"`
	Controller   *terminalControllerPayload `json:"controller"`
	Lease        terminalpkg.LeaseState     `json:"lease"`
	Viewers      int                        `json:"viewers"`
	BoundRun     *terminalRunPayload        `json:"bound_run"`
	Capabilities terminalpkg.Capabilities   `json:"capabilities"`
	CreatedAt    time.Time                  `json:"created_at"`
	Exit         *terminalExitPayload       `json:"exit,omitempty"`
}

func terminalInfoFromDomain(info terminalpkg.Info, profileName string) terminalInfoPayload {
	payload := terminalInfoPayload{
		ID: info.ID, WorkspaceID: info.WS, ProfileID: info.ProfileID, ProfileName: profileName,
		Title: info.Title, Shell: info.Shell, Cwd: info.Cwd, Mode: info.Mode, State: info.State,
		Lease: info.Lease, Viewers: info.Viewers, Capabilities: info.Capabilities, CreatedAt: info.CreatedAt,
	}
	if info.Controller != nil {
		payload.Controller = &terminalControllerPayload{Kind: info.Controller.Kind, ID: info.Controller.ID}
	}
	if info.BoundRun != nil {
		payload.BoundRun = &terminalRunPayload{SessionID: info.BoundRun.SessionID, RunID: info.BoundRun.RunID}
	}
	if info.Exit != nil {
		payload.Exit = terminalExitFromDomain(info.Exit)
	}
	return payload
}

func terminalExitFromDomain(exit *terminalpkg.Exit) *terminalExitPayload {
	if exit == nil {
		return nil
	}
	return &terminalExitPayload{Cause: exit.Cause, Code: exit.Code, Signal: exit.Signal, At: exit.At}
}
