package contract

import "github.com/compozy/compozy/internal/cmdpalette"

// CmdPaletteConfig declares commands and views contributed by an extension.
type CmdPaletteConfig struct {
	Commands []CmdPaletteCommand `toml:"commands,omitempty" json:"commands,omitempty"`
	Views    []CmdPaletteView    `toml:"views,omitempty"    json:"views,omitempty"`
}

// CmdPaletteCommand is one manifest-authored command before extension namespacing.
type CmdPaletteCommand struct {
	ID      string `toml:"id"                json:"id"`
	Title   string `toml:"title"             json:"title"`
	Section string `toml:"section,omitempty" json:"section,omitempty"`
	Icon    string `toml:"icon"              json:"icon"`
	Profile string `toml:"profile,omitempty" json:"profile,omitempty"`

	Keywords []string `toml:"keywords,omitempty" json:"keywords,omitempty"`

	Arguments []CmdPaletteArgument `toml:"arguments,omitempty" json:"arguments,omitempty"`

	Action CmdPaletteAction `toml:"action" json:"action"`

	Destructive  bool                    `toml:"destructive,omitempty"  json:"destructive,omitempty"`
	Confirmation *CmdPaletteConfirmation `toml:"confirmation,omitempty" json:"confirmation,omitempty"`

	DefaultShortcut string `toml:"default_shortcut,omitempty" json:"default_shortcut,omitempty"`

	Execution *CmdPaletteExecutionPolicy `toml:"execution,omitempty" json:"execution,omitempty"`
}

// CmdPaletteArgument describes one inline command argument.
type CmdPaletteArgument struct {
	Name        string   `toml:"name"                  json:"name"`
	Type        string   `toml:"type"                  json:"type"`
	Placeholder string   `toml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Required    bool     `toml:"required,omitempty"    json:"required,omitempty"`
	Options     []string `toml:"options,omitempty"     json:"options,omitempty"`
}

// CmdPaletteAction is the closed extension action union.
type CmdPaletteAction struct {
	Kind string         `toml:"kind"           json:"kind"`
	Tool string         `toml:"tool,omitempty" json:"tool,omitempty"`
	View string         `toml:"view,omitempty" json:"view,omitempty"`
	App  string         `toml:"app,omitempty"  json:"app,omitempty"`
	URL  string         `toml:"url,omitempty"  json:"url,omitempty"`
	Args map[string]any `toml:"args,omitempty" json:"args,omitempty"`
}

// CmdPaletteConfirmation is required for destructive commands.
type CmdPaletteConfirmation struct {
	Title   string `toml:"title"          json:"title"`
	Body    string `toml:"body,omitempty" json:"body,omitempty"`
	Confirm string `toml:"confirm"        json:"confirm"`
}

// CmdPaletteExecutionPolicy allows an author to override action-kind defaults.
type CmdPaletteExecutionPolicy struct {
	SingleFlight *bool `toml:"single_flight,omitempty" json:"single_flight,omitempty"`
	RetrySafe    *bool `toml:"retry_safe,omitempty"    json:"retry_safe,omitempty"`
}

// CmdPaletteView declares one declarative or programmable palette view.
type CmdPaletteView struct {
	ID      string `toml:"id"    json:"id"`
	Title   string `toml:"title" json:"title"`
	Kind    string `toml:"kind"  json:"kind"`
	Profile string `toml:"profile,omitempty" json:"profile,omitempty"`

	Source *CmdPaletteViewSource `toml:"source,omitempty" json:"source,omitempty"`

	Program bool `toml:"program,omitempty" json:"program,omitempty"`
}

// CmdPaletteViewSource binds a declarative view to one extension-owned tool.
type CmdPaletteViewSource struct {
	Tool string `toml:"tool" json:"tool"`
}

// ViewFrame is the canonical programmable-view frame shared with SDK codegen.
type ViewFrame = cmdpalette.ViewFrame
