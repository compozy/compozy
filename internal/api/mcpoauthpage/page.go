package mcpoauthpage

import (
	"fmt"
	"html/template"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

// Outcome is what the callback did, as the person in the browser needs to know it.
type Outcome string

const (
	// OutcomeConnected means the token was stored and the server is usable.
	OutcomeConnected Outcome = "connected"
	// OutcomeDeclined means the provider redirected back with an OAuth error.
	OutcomeDeclined Outcome = "declined"
	// OutcomeExpired means the sign-in no longer exists: past its TTL or already used.
	OutcomeExpired Outcome = "expired"
	// OutcomeChanged means the server definition changed while the sign-in was open.
	OutcomeChanged Outcome = "changed"
	// OutcomeUnavailable means the runtime could not take the callback yet.
	OutcomeUnavailable Outcome = "unavailable"
	// OutcomeFailed is every other failure; Diagnostic carries the reason.
	OutcomeFailed Outcome = "failed"
)

// openURL opens the MCP servers settings in the desktop app, where every sign-in starts.
const openURL = "compozyos://open/settings/mcp"

// span is the number of points on each side of the handshake mark.
const span = 9

// Server identifies the MCP server the sign-in was for.
type Server struct {
	Name string
	// Host is the host of the server's remote URL.
	Host string
}

// Page is one rendered outcome. Server is nil when the callback no longer resolves to a sign-in.
type Page struct {
	Outcome     Outcome
	Server      *Server
	Scope       mcpauth.Scope
	ProjectName string
	Scopes      []string
	Refreshable bool
	ExpiresAt   *time.Time
	// Code is the machine-readable reason shown under a failure message.
	Code string
	// Diagnostic is a copyable summary for unclassified failures; it never holds raw error text.
	Diagnostic string
	Now        time.Time
}

type view struct {
	Outcome      Outcome
	Tone         string
	Connected    bool
	Server       *Server
	Monogram     string
	Eyebrow      string
	Title        string
	Detail       string
	Code         string
	Availability string
	Scopes       []string
	Renewal      string
	Diagnostic   string
	StatusWord   string
	StatusHint   string
	Time         string
	SelfPoints   []bool
	ServerPoints []bool
	Mark         template.HTML
	OpenURL      template.URL
	Symbol       template.URL
	Converge     template.URL
	Horizon      template.URL
	CSS          template.CSS
	Script       template.JS
}

func newView(page Page) view {
	outcome := page.Outcome
	connected := outcome == OutcomeConnected
	server := page.Server
	if server != nil && strings.TrimSpace(server.Name) == "" {
		server = nil
	}
	title, detail := copyFor(outcome, server)
	result := view{
		Outcome:      outcome,
		Tone:         toneFor(outcome),
		Connected:    connected,
		Server:       server,
		Eyebrow:      "MCP server sign-in",
		Title:        title,
		Detail:       detail,
		Code:         page.Code,
		Diagnostic:   page.Diagnostic,
		StatusWord:   statusWordFor(outcome),
		StatusHint:   "Nothing was saved",
		Time:         page.Now.Format("3:04 PM"),
		SelfPoints:   points(outcome != OutcomeExpired && outcome != OutcomeUnavailable),
		ServerPoints: points(connected),
		Mark:         markFor(outcome),
		OpenURL:      openURL,
		Symbol:       template.URL(symbolURI),   //nolint:gosec // Embedded asset, not user input.
		Converge:     template.URL(convergeURI), //nolint:gosec // Embedded asset, not user input.
		Horizon:      template.URL(horizonURI),  //nolint:gosec // Embedded asset, not user input.
		CSS:          template.CSS(pageCSS),     //nolint:gosec // Embedded stylesheet, hashed in the CSP.
		Script:       template.JS(pageScript),   //nolint:gosec // Embedded script, hashed in the CSP.
	}
	if server != nil {
		result.Monogram = monogram(server.Name)
		result.Eyebrow = server.Name + " · MCP server"
	}
	if connected {
		result.StatusHint = "This tab can be closed"
		result.Availability = availability(page.Scope, page.ProjectName)
		result.Scopes = page.Scopes
		result.Renewal = renewal(page.Refreshable, page.ExpiresAt, page.Now)
	}
	return result
}

func copyFor(outcome Outcome, server *Server) (string, string) {
	name, known := "the server", false
	if server != nil {
		name, known = server.Name, true
	}
	switch outcome {
	case OutcomeConnected:
		if !known {
			return "The server is connected", "Agents in CompozyOS can use its tools now. You can close this tab."
		}
		return name + " is connected",
			"Agents in CompozyOS can use " + name + "’s tools now. You can close this tab."
	case OutcomeDeclined:
		title := "Access wasn’t granted"
		if known {
			title = name + " didn’t grant access"
		}
		return title, "The sign-in was canceled or declined on " + name +
			". Nothing changed in CompozyOS. Start again from the server’s settings when you’re ready."
	case OutcomeExpired:
		return "This sign-in link has expired", fmt.Sprintf(
			"Sign-in links last %d minutes and work only once. "+
				"Start a new sign-in from the server’s settings in CompozyOS.",
			int(mcpauth.DefaultLoginSessionTTL/time.Minute),
		)
	case OutcomeChanged:
		title := "The server changed while you signed in"
		if known {
			title = name + " changed while you signed in"
		}
		return title, "The server’s settings were edited during sign-in, so this one was set aside " +
			"to keep the new settings. Sign in again to finish."
	case OutcomeUnavailable:
		return "CompozyOS couldn’t finish signing in", "The runtime wasn’t ready to take this sign-in. " +
			"Open CompozyOS, wait until it has started, and sign in again."
	default:
		return "Sign-in didn’t finish", "CompozyOS couldn’t finish signing in to " + name +
			". Try again, or use manual sign-in from the server’s settings."
	}
}

func toneFor(outcome Outcome) string {
	switch outcome {
	case OutcomeConnected:
		return "success"
	case OutcomeDeclined, OutcomeExpired, OutcomeChanged:
		return "warning"
	default:
		return "danger"
	}
}

func statusWordFor(outcome Outcome) string {
	switch outcome {
	case OutcomeConnected:
		return "Connected"
	case OutcomeExpired:
		return "Link expired"
	case OutcomeFailed:
		return "Failed"
	default:
		return "Not connected"
	}
}

func markFor(outcome Outcome) template.HTML {
	switch outcome {
	case OutcomeConnected:
		return `<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M4 8.4l2.6 2.6L12 5.4" ` +
			`stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`
	case OutcomeExpired:
		return `<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><circle cx="8" cy="8" r="5" ` +
			`stroke="currentColor" stroke-width="1.6"/><path d="M8 5.4V8l1.8 1.2" stroke="currentColor" ` +
			`stroke-width="1.6" stroke-linecap="round"/></svg>`
	default:
		return `<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M5.2 5.2l5.6 5.6M10.8 5.2l-5.6 5.6" ` +
			`stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>`
	}
}

func points(lit bool) []bool {
	result := make([]bool, span)
	for index := range result {
		result[index] = lit
	}
	return result
}

func monogram(name string) string {
	for _, r := range strings.TrimSpace(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	if r, _ := utf8.DecodeRuneInString(strings.TrimSpace(name)); r != utf8.RuneError {
		return string(r)
	}
	return "?"
}

func availability(scope mcpauth.Scope, projectName string) string {
	project := "Project · " + projectName
	if strings.TrimSpace(projectName) == "" {
		project = "This project"
	}
	switch scope {
	case mcpauth.ScopeUser:
		return "All projects"
	case mcpauth.ScopeProfile:
		return "All projects in this profile"
	case mcpauth.ScopeWorkspace:
		return project
	case mcpauth.ScopeWorkspaceProfile:
		return project + ", this profile"
	default:
		return ""
	}
}

func renewal(refreshable bool, expiresAt *time.Time, now time.Time) string {
	if refreshable {
		return "Renews on its own"
	}
	if expiresAt == nil {
		return ""
	}
	left := expiresAt.Sub(now)
	switch {
	case left < time.Minute:
		return "Expires in less than a minute"
	case left < time.Hour:
		return plural(int(left/time.Minute), "minute")
	case left < 48*time.Hour:
		return plural(int(left/time.Hour), "hour")
	default:
		return "Expires on " + expiresAt.In(now.Location()).Format("Jan 2")
	}
}

func plural(count int, unit string) string {
	if count == 1 {
		return fmt.Sprintf("Expires in 1 %s", unit)
	}
	return fmt.Sprintf("Expires in %d %ss", count, unit)
}
