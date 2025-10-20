# Outside-of-diff for `engine/infra/server/router/helpers.go`

## Outside-of-diff from Comment 4

**File:** `engine/infra/server/router/helpers.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
> <summary>engine/infra/server/router/helpers.go (1)</summary><blockquote>
> 
> `120-127`: **Avoid duplicate/leaky "detail" in extras; use the code helper.**
> 
> Extras["detail"] will surface as an extra field and duplicate the canonical "details". Prefer the helper and drop the extra.
> 
> Apply this diff:
> 
> ```diff
> -    RespondProblem(c, &core.Problem{
> -      Status: http.StatusBadRequest,
> -      Detail: "invalid input",
> -      Extras: map[string]any{
> -        "code":   ErrBadRequestCode,
> -        "detail": err.Error(),
> -      },
> -    })
> +    RespondProblemWithCode(c, http.StatusBadRequest, ErrBadRequestCode, err.Error())
> ```
> As per coding guidelines.
> 
> </blockquote></details>
