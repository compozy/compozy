# Outside-of-diff from Comment 3

**File:** `internal/setup/reusable_agents.go`
**Date:** 2026-04-13 19:08:31 UTC
**Status:** - [x] RESOLVED

## Details

<details>
> <summary>internal/setup/reusable_agents.go (1)</summary><blockquote>
> 
> `56-75`: _⚠️ Potential issue_ | _🟡 Minor_
> 
> **Move reusable-agent name validation to the beginning of `parseReusableAgent`.**
> 
> The `validateReusableAgentName(dir)` call at line 73 runs after `path.Join` and `fs.ReadFile`, allowing unnecessary filesystem access attempts for invalid names. Move the validation to the top of the function to fail fast.
> 
> <details>
> <summary>♻️ Suggested fix</summary>
> 
> ```diff
>  func parseReusableAgent(bundle fs.FS, dir string) (ReusableAgent, error) {
> +	if err := validateReusableAgentName(dir); err != nil {
> +		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
> +	}
> +
>  	agentPath := path.Join(dir, "AGENT.md")
>  	content, err := fs.ReadFile(bundle, agentPath)
>  	if err != nil {
>  		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
>  	}
> @@
> -	if err := validateReusableAgentName(dir); err != nil {
> -		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
> -	}
> -
>  	return ReusableAgent{
> ```
> </details>
> 
> <details>
> <summary>🤖 Prompt for AI Agents</summary>
> 
> ```
> Verify each finding against the current code and only fix it if needed.
> 
> In `@internal/setup/reusable_agents.go` around lines 56 - 75, Validate the
> reusable-agent directory name at the start of parseReusableAgent by calling
> validateReusableAgentName(dir) before constructing agentPath or calling
> fs.ReadFile(bundle, agentPath); if validation fails return the error as before.
> Move the existing validateReusableAgentName(dir) check to the top of
> parseReusableAgent (before agentPath and ReadFile) so invalid names fail fast
> while keeping the same error wrapping behavior.
> ```
> 
> </details>
> 
> </blockquote></details>

## Triage

- Disposition: VALID
- Resolution: moved `validateReusableAgentName(dir)` to the top of `parseReusableAgent` so invalid reusable-agent names fail before any filesystem read is attempted, while preserving the existing error wrapping.
