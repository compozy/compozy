# Issue 13 - Review Thread Comment

**File:** `pkg/config/loader.go:447`
**Date:** 2025-10-27 13:58:52 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_🛠️ Refactor suggestion_ | _🟠 Major_

**Replace remaining 65535 literals with maxTCPPort; unify messages.**

Use the defined maxTCPPort for range checks and format error messages with %d to avoid magic numbers and keep a single source of truth.

Apply:

```diff
-	if standalone.FrontendPort < 1 || standalone.FrontendPort > maxTCPPort {
-		return fmt.Errorf("temporal.standalone.frontend_port must be between 1 and 65535")
+	if standalone.FrontendPort < 1 || standalone.FrontendPort > maxTCPPort {
+		return fmt.Errorf("temporal.standalone.frontend_port must be between 1 and %d", maxTCPPort)
 	}
@@
-		if standalone.UIPort < 1 || standalone.UIPort > maxTCPPort {
-			return fmt.Errorf("temporal.standalone.ui_port must be between 1 and 65535 when enable_ui is true")
+		if standalone.UIPort < 1 || standalone.UIPort > maxTCPPort {
+			return fmt.Errorf("temporal.standalone.ui_port must be between 1 and %d when enable_ui is true", maxTCPPort)
 		}
@@
-	} else if standalone.UIPort != 0 && (standalone.UIPort < 1 || standalone.UIPort > 65535) {
-		return fmt.Errorf("temporal.standalone.ui_port must be between 1 and 65535 when set")
+	} else if standalone.UIPort != 0 && (standalone.UIPort < 1 || standalone.UIPort > maxTCPPort) {
+		return fmt.Errorf("temporal.standalone.ui_port must be between 1 and %d when set", maxTCPPort)
 	}
```

And in validateTCPPort:

```diff
-	if port < 1 || port > 65535 {
-		return fmt.Errorf("%s must be between 1 and 65535, got: %d", fieldName, port)
+	if port < 1 || port > maxTCPPort {
+		return fmt.Errorf("%s must be between 1 and %d, got: %d", fieldName, maxTCPPort, port)
 	}
```
As per coding guidelines.


Also applies to: 594-604

<!-- fingerprinting:phantom:medusa:chinchilla -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5fez8F`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5fez8F
```

---
*Generated from PR review - CodeRabbit AI*
