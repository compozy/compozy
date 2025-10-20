# Outside-of-diff for `engine/infra/server/middleware/auth/middleware.go`

## Outside-of-diff from Comment 4

**File:** `engine/infra/server/middleware/auth/middleware.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
> <summary>engine/infra/server/middleware/auth/middleware.go (1)</summary><blockquote>
> 
> `251-270`: **RequireAuth/RequireAdmin should also use router.RespondWithError**
> 
> Keep all error responses consistent with router helpers.
> 
> 
> ```diff
> @@
> -    if _, ok := userctx.UserFromContext(c.Request.Context()); !ok {
> -      c.JSON(401, gin.H{"error": "Authentication required", "details": "This endpoint requires a valid API key"})
> -      c.Abort()
> -      return
> -    }
> +    if _, ok := userctx.UserFromContext(c.Request.Context()); !ok {
> +      router.RespondWithError(c, http.StatusUnauthorized,
> +        router.NewRequestError(http.StatusUnauthorized, "Authentication required",
> +          fmt.Errorf("this endpoint requires a valid API key")))
> +      c.Abort()
> +      return
> +    }
> @@
> -    if !ok || user.Role != model.RoleAdmin {
> -      c.JSON(403, gin.H{"error": "Admin access required", "details": "This endpoint requires admin privileges"})
> -      c.Abort()
> -      return
> -    }
> +    if !ok || user.Role != model.RoleAdmin {
> +      router.RespondWithError(c, http.StatusForbidden,
> +        router.NewRequestError(http.StatusForbidden, "Admin access required",
> +          fmt.Errorf("this endpoint requires admin privileges")))
> +      c.Abort()
> +      return
> +    }
> ```
> Note: add imports:
> 
> ```diff
> + "net/http"
> + "github.com/compozy/compozy/engine/infra/server/router"
> + "fmt"
> ```
> 
> 
> Also applies to: 272-292
> 
> </blockquote></details>
