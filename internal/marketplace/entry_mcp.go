package marketplace

import "fmt"

type mcpEntry struct {
	entryCommon
	Launch       mcpLaunch  `json:"launch"`
	Auth         *mcpAuth   `json:"auth,omitempty"`
	Inputs       []mcpInput `json:"inputs,omitempty"`
	DefaultScope string     `json:"default_scope"`
}

type mcpAuth struct {
	Method       string   `json:"method"`
	Registration string   `json:"registration"`
	Scopes       []string `json:"scopes,omitempty"`
}

func decodeMCPEntry(raw []byte) (Entry, error) {
	var value mcpEntry
	if err := decodeStrict(raw, &value); err != nil {
		return Entry{}, err
	}
	if err := value.validate(); err != nil {
		return Entry{}, err
	}
	return commonEntry(KindMCP, value.entryCommon, value)
}

func (e mcpEntry) validate() error {
	if err := e.entryCommon.validate(KindMCP); err != nil {
		return err
	}
	if err := e.Launch.validate(e.EntryID); err != nil {
		return err
	}
	if err := e.validateAuth(); err != nil {
		return err
	}
	if err := validateMCPInputs(e.EntryID, e.Launch, e.Inputs); err != nil {
		return err
	}
	return e.validateDefaultScope()
}

func (e mcpEntry) validateAuth() error {
	if e.Auth == nil {
		return nil
	}
	if e.Launch.Type != mcpLaunchTypeRemote {
		return fmt.Errorf("marketplace catalog MCP entry %q auth is only supported for remote launch", e.EntryID)
	}
	return e.Auth.validate(e.EntryID)
}

func (e mcpEntry) validateDefaultScope() error {
	switch e.DefaultScope {
	case "workspace", "global":
		return nil
	default:
		return fmt.Errorf("marketplace catalog MCP entry %q default_scope must be workspace or global", e.EntryID)
	}
}
