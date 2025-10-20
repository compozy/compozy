# Outside-of-diff for `engine/agent/router/agents_top.go`

## Outside-of-diff from Comment 4

**File:** `engine/agent/router/agents_top.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
> <summary>engine/agent/router/agents_top.go (1)</summary><blockquote>
> 
> `180-191`: **Use router.GetRequestBody for consistency and shorter code.**
> 
> Avoid manual bind and unify error handling with standardized problem responses.
> 
> ```diff
> -  body := make(map[string]any)
> -  if err := c.ShouldBindJSON(&body); err != nil {
> -    router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
> -    return
> -  }
> +  body := router.GetRequestBody[map[string]any](c)
> +  if body == nil {
> +    return
> +  }
> @@
> -  input := &agentuc.UpsertInput{Project: project, ID: agentID, Body: body, IfMatch: ifMatch}
> +  input := &agentuc.UpsertInput{Project: project, ID: agentID, Body: *body, IfMatch: ifMatch}
> ```
> As per coding guidelines.
> 
> </blockquote></details>
