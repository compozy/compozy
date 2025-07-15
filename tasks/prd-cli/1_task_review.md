🔴 HIGH SEVERITY ISSUES (2)

1. Rate Limiting Not Implemented

File: /cli/api*client.go:78-84
// Rate limiting configuration
if cfg.RateLimit.GlobalRate.Limit > 0 {
// Add rate limiting middleware
client.OnBeforeRequest(func(* *resty.Client, \_ *resty.Request) error {
// Rate limiting logic can be added here
return nil
})
}
Issue: Stub implementation - no actual rate limiting despite configuration support
Impact: Could lead to API abuse, 429 errors, potential IP bans
Fix: Implement token bucket algorithm using golang.org/x/time/rate

2. Nil Pointer Risk in Retry Logic (Expert finding - validated)

File: /cli/api_client.go:67-70
AddRetryCondition(func(r \*resty.Response, err error) bool {
return err != nil || (r.StatusCode() >= 500 && r.StatusCode() < 600)
})
Issue: Could panic if r is nil when checking StatusCode
Fix: Add nil check before accessing response

🟡 MEDIUM SEVERITY ISSUES (6)

1. Incomplete Retry Status Codes

File: /cli/api_client.go:67-70
Missing: 429 (Rate Limit), 408 (Timeout), 502-504 (Gateway errors)
Fix: Extend retry condition to include these transient errors

2-6. Function Length Violations (30-line limit)

- api_client.go:34-103 (70 lines): NewAPIClient
- api_client.go:135-183 (49 lines): doRequest
- output.go:456-487 (32 lines): isRunningInCI
- output.go:490-522 (33 lines): checkExplicitFlags
- command_executor.go:850-889 (40 lines): HandleCommonErrors
- utils/cliutils.go:156-192 (37 lines): formatErrorTUI

🟢 LOW SEVERITY ISSUES (6)

1. Unchecked Type Assertions - Could cause panics in error paths
2. Error Detection via String Matching - Brittle, locale-dependent
3. TODO Comments - SSE implementation, API enhancements
4. Tag Serialization (Expert finding) - fmt.Sprintf("%v", tags) produces [tag1 tag2]
5. Base URL Validation (Expert finding) - Accepts relative URLs
6. Test Mock Type Assertions - Missing safety checks
