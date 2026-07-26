package scan

import (
	"fmt"
	"regexp"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	redactpkg "github.com/compozy/agh/internal/redact"
)

// Action is the strongest deterministic outcome produced by the scanner.
type Action string

const (
	// ActionAllow means no scan rule matched.
	ActionAllow Action = "allow"
	// ActionAnnotate means the content may continue with a safe policy note.
	ActionAnnotate Action = "annotate"
	// ActionReject means the content must not be persisted.
	ActionReject Action = "reject"
)

// Category groups scan matches by policy family.
type Category string

const (
	// CategoryThreat covers prompt-injection, exfiltration, and persistence payloads.
	CategoryThreat Category = "threat"
	// CategoryWhatNotToSave covers Slice 1 persistence denylist policy.
	CategoryWhatNotToSave Category = "what_not_to_save"
	// CategoryAnnotation covers non-blocking policy hints for later controller tasks.
	CategoryAnnotation Category = "annotation"
)

// Match describes one deterministic rule hit without exposing the matched content.
type Match struct {
	RuleID   string
	Category Category
	Action   Action
	Reason   string
}

// Result is the redaction-safe outcome of scanning candidate memory content.
type Result struct {
	Action  Action
	Matches []Match
}

type contentRule struct {
	id       string
	category Category
	action   Action
	reason   string
	pattern  *regexp.Regexp
}

type runeRule struct {
	id     string
	r      rune
	reason string
}

var invisibleRuneRules = []runeRule{
	{id: "invisible_unicode_u_200b", r: '\u200b', reason: "contains invisible Unicode control U+200B"},
	{id: "invisible_unicode_u_200c", r: '\u200c', reason: "contains invisible Unicode control U+200C"},
	{id: "invisible_unicode_u_200d", r: '\u200d', reason: "contains invisible Unicode control U+200D"},
	{id: "invisible_unicode_u_2060", r: '\u2060', reason: "contains invisible Unicode control U+2060"},
	{id: "invisible_unicode_u_feff", r: '\ufeff', reason: "contains invisible Unicode control U+FEFF"},
	{id: "invisible_unicode_u_202a", r: '\u202a', reason: "contains bidi control U+202A"},
	{id: "invisible_unicode_u_202b", r: '\u202b', reason: "contains bidi control U+202B"},
	{id: "invisible_unicode_u_202c", r: '\u202c', reason: "contains bidi control U+202C"},
	{id: "invisible_unicode_u_202d", r: '\u202d', reason: "contains bidi control U+202D"},
	{id: "invisible_unicode_u_202e", r: '\u202e', reason: "contains bidi control U+202E"},
}

var contentRules = []contentRule{
	{
		id:       "prompt_injection_ignore_previous",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains prompt-injection override language",
		pattern:  regexp.MustCompile(`(?i)\bignore\s+(?:all\s+)?(?:previous|above|prior)\s+instructions?\b`),
	},
	{
		id:       "prompt_injection_disregard_policy",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains prompt-injection disregard language",
		pattern: regexp.MustCompile(
			`(?i)\bdisregard\s+(?:all\s+)?(?:(?:previous|above|prior)\s+)?(?:instructions?|rules|guidelines)\b`,
		),
	},
	{
		id:       "prompt_injection_role_override",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains role-override language",
		pattern:  regexp.MustCompile(`(?i)\byou\s+are\s+now\b`),
	},
	{
		id:       "prompt_injection_hidden_instruction",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains instruction-hiding language",
		pattern:  regexp.MustCompile(`(?i)\bdo\s+not\s+tell\s+the\s+user\b`),
	},
	{
		id:       "prompt_injection_system_override",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains system-prompt override language",
		pattern:  regexp.MustCompile(`(?i)\bsystem\s+prompt\s+override\b`),
	},
	{
		id:       "prompt_injection_restriction_bypass",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains restriction-bypass language",
		pattern: regexp.MustCompile(
			`(?i)\bact\s+as\s+(?:if|though)\s+you\s+(?:have\s+no|don't\s+have)\s+(?:restrictions?|limits?|rules)\b`,
		),
	},
	{
		id:       "exfiltration_curl_wget_secret",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains secret-exfiltration command language",
		pattern: regexp.MustCompile(
			`(?i)\b(?:curl|wget)\b[^\n]*(?:api[_-]?key|token|secret|password|credential|openai|anthropic|github)[^\n]*`,
		),
	},
	{
		id:       "exfiltration_cat_sensitive_file",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains sensitive-file read command language",
		pattern: regexp.MustCompile(
			`(?i)\bcat\s+(?:~?/)?(?:\.ssh|\.env|/etc/(?:passwd|shadow)|[^\n]*(?:secret|credential|token|password)[^\s]*)`,
		),
	},
	{
		id:       "exfiltration_netcat_exec",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains netcat exec command language",
		pattern:  regexp.MustCompile(`(?i)\b(?:nc|netcat)\b[^\n]*(?:-e|--exec)\b`),
	},
	{
		id:       "exfiltration_base64_pipe",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains encoded payload command language",
		pattern:  regexp.MustCompile(`(?i)\bbase64\s+-d\b[^\n]*(?:\||>|>>)`),
	},
	{
		id:       "persistence_authorized_keys",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains SSH persistence language",
		pattern:  regexp.MustCompile(`(?i)\bauthorized_keys\b`),
	},
	{
		id:       "persistence_ssh_directory",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains SSH persistence path language",
		pattern:  regexp.MustCompile(`(?i)(?:^|[\s/])\.ssh(?:[\s/]|$)`),
	},
	{
		id:       "persistence_launchctl",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains launch-agent persistence language",
		pattern:  regexp.MustCompile(`(?i)\blaunchctl\b`),
	},
	{
		id:       "persistence_cron",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains cron persistence language",
		pattern:  regexp.MustCompile(`(?i)\b(?:crontab|cron)\b`),
	},
	{
		id:       "persistence_systemd",
		category: CategoryThreat,
		action:   ActionReject,
		reason:   "contains systemd persistence language",
		pattern:  regexp.MustCompile(`(?i)\b(?:systemctl|systemd)\b`),
	},
	{
		id:       "policy_code_block",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE code-block policy",
		pattern:  regexp.MustCompile("(?m)^```"),
	},
	{
		id:       "policy_code_declaration",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE code-pattern policy",
		pattern:  regexp.MustCompile(`(?m)^\s*(?:package|import|func|class|def|interface|type|const|var)\s+[A-Za-z_]`),
	},
	{
		id:       "policy_repo_path",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE repository-derived policy",
		pattern: regexp.MustCompile(
			`(?i)\b(?:cmd|internal|web|packages|sdk|openapi|docs|scripts|\.compozy)/[A-Za-z0-9._/-]+`,
		),
	},
	{
		id:       "policy_debugging_session",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE debugging/session policy",
		pattern: regexp.MustCompile(
			`(?i)\b(?:stack trace|failing tests?|root cause|workaround|regression|panic trace)\b`,
		),
	},
	{
		id:       "policy_ephemeral_task_state",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE ephemeral task-state policy",
		pattern: regexp.MustCompile(
			`(?i)\b(?:current task|in progress|next steps?|this session|just ran|today'?s operational status|activity summary|PR list|latest assistant message)\b`,
		),
	},
	{
		id:       "policy_repository_documentation",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE already-documented policy",
		pattern: regexp.MustCompile(
			`(?i)\b(?:AGENTS\.md|CLAUDE\.md|docs/_memory/standing_directives\.md|standing directives|ADR-\d{3}|_techspec\.md|_tasks\.md)\b`,
		),
	},
	{
		id:       "policy_transcript_dump",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE transcript-dump policy",
		pattern:  regexp.MustCompile(`(?mi)(^\s*(?:user|assistant|system|tool):|\btranscript dump\b)`),
	},
	{
		id:       "policy_secret_material",
		category: CategoryWhatNotToSave,
		action:   ActionReject,
		reason:   "matches WHAT_NOT_TO_SAVE secret-material policy",
		pattern: regexp.MustCompile(
			`(?i)\b(?:api[_-]?key|secret|password|credential|private key|\.env)\b|\b(?:token|secret|password|credential|api[_-]?key)\s*[:=]|sk-[A-Za-z0-9]{8,}`,
		),
	},
	{
		id:       "annotation_relative_time",
		category: CategoryAnnotation,
		action:   ActionAnnotate,
		reason:   "contains relative-time language that may be non-durable",
		pattern:  regexp.MustCompile(`(?i)\b(?:today|yesterday|tomorrow|current sprint|this week|next week)\b`),
	},
}

// Candidate scans the candidate content before persistence.
func Candidate(candidate memcontract.Candidate) Result {
	return Content(candidate.Content)
}

// Content scans memory content with deterministic lexical policy rules.
func Content(content string) Result {
	result := Result{Action: ActionAllow}
	if redactpkg.ContainsRawClaimToken(content) {
		result.add(Match{
			RuleID:   "policy_raw_claim_token",
			Category: CategoryWhatNotToSave,
			Action:   ActionReject,
			Reason:   "matches WHAT_NOT_TO_SAVE raw claim-token policy",
		})
	}
	for _, rule := range invisibleRuneRules {
		if strings.ContainsRune(content, rule.r) {
			result.add(Match{
				RuleID:   rule.id,
				Category: CategoryThreat,
				Action:   ActionReject,
				Reason:   rule.reason,
			})
		}
	}
	for _, rule := range contentRules {
		if rule.pattern.MatchString(content) {
			result.add(Match{
				RuleID:   rule.id,
				Category: rule.category,
				Action:   rule.action,
				Reason:   rule.reason,
			})
		}
	}
	return result
}

// Allowed reports whether the scan result may continue to later write decisions.
func (r Result) Allowed() bool {
	return r.Action != ActionReject
}

// Rejected reports whether the scan result must block persistence.
func (r Result) Rejected() bool {
	return r.Action == ActionReject
}

// Reason returns a redaction-safe explanation that never includes scanned content.
func (r Result) Reason() string {
	if len(r.Matches) == 0 {
		return "memory content passed deterministic scan"
	}
	ruleIDs := make([]string, 0, len(r.Matches))
	for _, match := range r.Matches {
		ruleIDs = append(ruleIDs, match.RuleID)
	}
	return fmt.Sprintf("memory content %s by scan rules: %s", actionVerb(r.Action), strings.Join(ruleIDs, ", "))
}

// RuleHits converts scan matches to controller rule-trace entries.
func (r Result) RuleHits() []memcontract.RuleHit {
	hits := make([]memcontract.RuleHit, 0, len(r.Matches))
	for _, match := range r.Matches {
		hits = append(hits, memcontract.RuleHit{
			Name:    "memory_scan." + match.RuleID,
			Passed:  false,
			Reason:  match.Reason,
			Target:  string(match.Category),
			Details: string(match.Action),
		})
	}
	return hits
}

func (r *Result) add(match Match) {
	r.Matches = append(r.Matches, match)
	if actionPriority(match.Action) > actionPriority(r.Action) {
		r.Action = match.Action
	}
}

func actionPriority(action Action) int {
	switch action {
	case ActionReject:
		return 2
	case ActionAnnotate:
		return 1
	default:
		return 0
	}
}

func actionVerb(action Action) string {
	switch action {
	case ActionReject:
		return "rejected"
	case ActionAnnotate:
		return "annotated"
	default:
		return "allowed"
	}
}
