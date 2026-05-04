---
status: resolved
file: internal/api/httpapi/dev_proxy.go
line: 38
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc58jarj,comment:PRRC_kwDORy7nkc655CwL
---

# Issue 002: _⚠️ Potential issue_ | _🟠 Major_
## Review Comment

_⚠️ Potential issue_ | _🟠 Major_

<details>
<summary>🧩 Analysis chain</summary>

🏁 Script executed:

```shell
find . -name "dev_proxy.go" -type f
```

Repository: compozy/compozy

Length of output: 94

---

🏁 Script executed:

```shell
git ls-files | grep -E "(dev_proxy|httpapi)" | head -20
```

Repository: compozy/compozy

Length of output: 448

---

🏁 Script executed:

```shell
cat -n internal/api/httpapi/dev_proxy.go
```

Repository: compozy/compozy

Length of output: 2569

---

🌐 Web query:

`httputil.ReverseProxy headers forwarding behavior Go standard library`

💡 Result:

The httputil.ReverseProxy in Go's net/http/httputil package forwards most incoming request headers to the backend server, with specific behaviors and modifications: Request Headers (to backend): - Forwards all non-hop-by-hop headers from the incoming request. - Automatically removes hop-by-hop headers (per RFC 9110 section 7.6.1): Connection, Proxy-Connection, Keep-Alive, Proxy-Authenticate, Proxy-Authentication, TE, Trailer, Transfer-Encoding, Upgrade. These are stripped after Director/Rewrite but can be added back in Rewrite (Go 1.20+). - Preserves and appends to X-Forwarded-For with the proxy's client IP (from req.RemoteAddr) by default when using Director (deprecated). To prevent spoofing, delete incoming X-Forwarded-* headers in Director/Rewrite. - In Go 1.20+, using Rewrite: X-Forwarded-*, X-Forwarded-Host, X-Forwarded-Proto, and Forwarded headers are automatically removed from outbound request before Rewrite for security. Use ProxyRequest.SetXForwarded to add them safely. - Headers added/modified in Director/Rewrite are preserved unless they are hop-by-hop (removed after). - Special case for Host: Must set both req.URL.Host and req.Host (not just Header.Set("Host")) in Director for correct forwarding. - User-Agent is cleared by default (set to empty). Response Headers (to client): - Forwards all non-hop-by-hop headers from backend. - Removes hop-by-hop headers before copying to client. - ModifyResponse func (if set) can adjust response headers. NewSingleHostReverseProxy uses Director internally (preserves incoming X-Forwarded-*). For modern usage, prefer direct ReverseProxy with Rewrite. Example (modern, Go 1.20+): proxy := &httputil.ReverseProxy{ Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(targetURL) r.Out.Host = r.In.Host // Preserve original Host if needed r.SetXForwarded // Add trusted X-Forwarded-* // Add custom headers r.Out.Header.Set("Authorization", "Bearer token") }, } Director example (older): proxy := &httputil.ReverseProxy{ Director: func(req *http.Request) { req.URL.Scheme = target.Scheme req.URL.Host = target.Host req.Host = target.Host delete(req.Header, "X-Forwarded-For") // Anti-spoofing req.Header.Set("X-Forwarded-For", clientIP) }, } Hop-by-hop removal uses removeHopByHopHeaders func, parsing Connection header (RFC 7230 compliant). As of Go 1.23+ (current in 2026), no major changes; Rewrite preferred over deprecated Director.

Citations:

- 1: https://go.dev/src/net/http/httputil/reverseproxy.go
- 2: https://go.dev/pkg/net/http/httputil
- 3: https://pkg.go.dev/net/http/httputil
- 4: https://pkg.go.dev/net/http/httputil@go1.25.6
- 5: https://go.dev/src/net/http/httputil/reverseproxy.go?h=ReverseProxy
- 6: https://pkg.go.dev/net/http/httputil@go1.22.4

---

🌐 Web query:

`Go net/http httputil ProxyRequest Out.Header Del method`

💡 Result:

In Go's net/http/httputil package (Go 1.20+), use ReverseProxy with a Rewrite function on *ProxyRequest to modify the outbound request's headers. ProxyRequest has fields In (*http.Request, read-only) and Out (*http.Request, modifiable). Access headers via r.Out.Header, which is http.Header supporting Del(key string) to remove a header. Example from official docs: proxy := &httputil.ReverseProxy{ Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target) // Modify outbound headers here: r.Out.Header.Del("Some-Header") // Removes "Some-Header" r.Out.Header.Set("New-Header", "value") }, } Before Rewrite is called, the library removes forwarding headers (Forwarded, X-Forwarded-*) and hop-by-hop headers from r.Out. You can add them back (e.g., via r.SetXForwarded) or delete others in Rewrite. Director is deprecated; use Rewrite instead. For older Go (<1.20), use Director on ReverseProxy and call outreq.Header.Del("header") there, but note hop-by-hop headers are stripped afterward. Code snippet for deleting a header: import ( "net/http" "net/http/httputil" "net/url" ) target, _ := url.Parse("http://backend.example.com") proxy := &httputil.ReverseProxy{ Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target) r.Out.Header.Del("Authorization") // Example: strip auth header }, } http.ListenAndServe(":8080", proxy) http.Header.Del removes all values for the key (case-insensitive).

Citations:

- 1: https://pkg.go.dev/net/http/httputil
- 2: https://go.dev/pkg/net/http/httputil
- 3: https://pkg.go.dev/net/http/httputil@go1.21.3
- 4: https://github.com/golang/go/blob/master/src/net/http/httputil/reverseproxy.go
- 5: https://pkg.go.dev/net/http/httputil@go1.22.4
- 6: https://oneuptime.com/blog/post/2026-01-25-reverse-proxy-request-rewriting-go/view
- 7: https://go.dev/src/net/http/httputil/reverseproxy.go?h=ReverseProxy
- 8: https://pkg.go.dev/net/http/httputil@go1.25.0

---

🏁 Script executed:

```shell
cat -n internal/api/httpapi/dev_proxy_test.go
```

Repository: compozy/compozy

Length of output: 8068

---



</details>

**Strip daemon auth/session headers before forwarding to the dev server.**

`httputil.ReverseProxy` forwards all non-hop-by-hop headers to the upstream server by default, including `Authorization`, `Cookie`, and CSRF tokens. These browser credentials, issued for the daemon, should not be forwarded to the Vite dev server, which does not need them and should not have access to daemon session material.



<details>
<summary>🔒 Suggested hardening</summary>

```diff
 		proxy: &httputil.ReverseProxy{
 			Rewrite: func(request *httputil.ProxyRequest) {
 				request.SetURL(target)
 				request.Out.Host = request.In.Host
 				request.SetXForwarded()
+				request.Out.Header.Del("Authorization")
+				request.Out.Header.Del("Cookie")
+				request.Out.Header.Del("X-CSRF-Token")
+				request.Out.Header.Del("X-Csrf-Token")
 			},
```
</details>

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/httpapi/dev_proxy.go` around lines 26 - 38, The dev proxy
currently forwards browser credentials; update the ReverseProxy Rewrite function
(the anonymous func on proxy: &httputil.ReverseProxy { Rewrite: func(request
*httputil.ProxyRequest) { ... } }) to strip sensitive daemon/session headers
before forwarding by deleting Authorization, Cookie, and CSRF-related headers
(e.g., "Authorization", "Cookie", "X-CSRF-Token", and any session cookies) from
request.Out.Header (and request.Out.Host if needed), then call
request.SetURL(target) / request.SetXForwarded() as before; ensure ErrorHandler
remains unchanged.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:2fe85954-b1fb-4290-8e9e-00be8dcc48f0 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - `httputil.ReverseProxy` preserves non-hop-by-hop headers by default, and the current `Rewrite` function only rewrites the target URL/forwarding headers.
  - The daemon’s browser auth material can therefore reach the Vite dev server via `Authorization`, `Cookie`, and the daemon CSRF header, which is a real credential-boundary violation.
  - Implemented: the proxy `Rewrite` path now strips `Authorization`, `Cookie`, and `core.HeaderCSRF` before forwarding, and `TestDevProxyRoutesStripDaemonCredentialsBeforeForwarding` proves the upstream sees none of them.
  - Verification: `go test ./internal/api/httpapi -run 'Test(DevProxyRoutesServeFrontendRequests|DevProxyRoutesStripDaemonCredentialsBeforeForwarding|DevProxyRoutesBypassAPIAndUnsupportedMethods|DevProxyReturnsBadGatewayWhenUpstreamIsUnavailable|NewWithDevProxyTargetPrefersProxyOverEmbeddedStaticFS|BrowserOpenAPIContractMatchesRegisteredBrowserRoutes)$' -count=1`
  - Repo gate note: the attempted full `make verify` run stopped earlier in `frontend:bootstrap` because of the unrelated pre-existing `package.json`/`bun.lock` mismatch.
