package core

import (
	"context"
	"errors"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/projectfiles/changes"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type SessionChangesReader interface{ changes.Engine }

func (h *BaseHandlers) changeRequest(c *gin.Context, details bool) (projectfiles.Root, changes.Request, bool) {
	root, ok := h.projectFileRoot(c)
	if !ok {
		return root, changes.Request{}, false
	}
	req := changes.Request{
		Scope:    changes.Scope(c.Query("scope")),
		BaseRef:  c.Query("base"),
		Revision: c.Query("revision"),
	}
	if raw := c.Query("reveal"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			h.changeError(c, projectfiles.ErrInvalidPath)
			return root, req, false
		}
		req.Reveal = v
	}
	if _, app := AppTokenFromContext(c.Request.Context()); app && req.Reveal {
		h.respondSessionFilesError(
			c,
			403,
			"reveal_forbidden",
			errors.New("sensitive content cannot be revealed by app tokens"),
		)
		return root, req, false
	}
	if details && req.Revision == "" {
		h.respondSessionFilesError(c, 400, "invalid_request", errors.New("revision is required"))
		return root, req, false
	}
	return root, req, true
}
func (h *BaseHandlers) GetSessionChanges(c *gin.Context) {
	root, req, ok := h.changeRequest(c, false)
	if !ok {
		return
	}
	s, err := h.SessionChanges.Summary(c.Request.Context(), root, req)
	if err != nil {
		h.changeError(c, err)
		return
	}
	response := SessionChangesPayload(root, s)
	if projection, ok := h.Sessions.(interface {
		SessionPullRequests(context.Context, []*session.Info) (map[string]session.PullRequestProjection, error)
	}); ok {
		info, err := h.Sessions.Status(c.Request.Context(), c.Param("session_id"))
		if err == nil {
			views, err := projection.SessionPullRequests(c.Request.Context(), []*session.Info{info})
			if err == nil {
				view := views[info.ID]
				response.PullRequest, _ = pullRequestPayload(view.PullRequestView)
			}
		}
	}
	c.JSON(200, response)
}
func (h *BaseHandlers) GetSessionChangePatches(c *gin.Context) {
	root, req, ok := h.changeRequest(c, true)
	if !ok {
		return
	}
	paths := c.QueryArray("path")
	old := c.QueryArray("old_path")
	if len(old) > 0 && len(old) != len(paths) {
		h.respondSessionFilesError(c, 400, "invalid_request", errors.New("old_path must align with path"))
		return
	}
	targets := make([]changes.PatchTarget, 0, len(paths))
	for i, p := range paths {
		t := changes.PatchTarget{Path: p}
		if len(old) > 0 {
			t.OldPath = old[i]
		}
		targets = append(targets, t)
	}
	patches, err := h.SessionChanges.Patches(c.Request.Context(), root, req, targets)
	if err != nil {
		h.changeError(c, err)
		return
	}
	result := contract.SessionChangePatchesResponse{Revision: req.Revision, Patches: []contract.ChangePatchPayload{}}
	result.Patches = ChangePatchPayloads(patches)
	c.JSON(200, result)
}
func (h *BaseHandlers) GetSessionChangeContents(c *gin.Context) {
	root, req, ok := h.changeRequest(c, true)
	if !ok {
		return
	}
	v, err := h.SessionChanges.Contents(
		c.Request.Context(),
		root,
		req,
		changes.PatchTarget{Path: c.Query("path"), OldPath: c.Query("old_path")},
	)
	if err != nil {
		h.changeError(c, err)
		return
	}
	c.JSON(
		200,
		contract.SessionChangeContentsResponse{
			Revision:    req.Revision,
			Path:        v.Path,
			OldPath:     v.OldPath,
			Fingerprint: v.Fingerprint,
			Old:         changeSide(v.Old),
			New:         changeSide(v.New),
		},
	)
}
func changeSide(s *changes.Side) *contract.ChangeSidePayload {
	if s == nil {
		return nil
	}
	v := &contract.ChangeSidePayload{Sensitive: s.Sensitive}
	if !s.Sensitive {
		v.Text = new(s.Text)
	}
	return v
}
func (h *BaseHandlers) ListSessionChangeBases(c *gin.Context) {
	root, _, ok := h.changeRequest(c, false)
	if !ok {
		return
	}
	limit := 0
	if raw := c.Query("limit"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 500 {
			h.respondSessionFilesError(c, 400, "invalid_request", errors.New("limit must be between 1 and 500"))
			return
		}
		limit = v
	}
	v, err := h.SessionChanges.Bases(c.Request.Context(), root, c.Query("q"), limit)
	if err != nil {
		h.changeError(c, err)
		return
	}
	result := contract.SessionChangeBasesResponse{
		Resolvable: v.Resolvable,
		Truncated:  v.Truncated,
		Refs:       []contract.ChangeBaseRefPayload{},
	}
	for _, r := range v.Refs {
		result.Refs = append(
			result.Refs,
			contract.ChangeBaseRefPayload{Ref: r.Ref, Kind: r.Kind, IsDefault: r.IsDefault, Source: string(r.Source)},
		)
	}
	if v.Default != nil {
		result.Default = &contract.ChangeBaseRefPayload{
			Ref:       v.Default.Ref,
			Kind:      v.Default.Kind,
			IsDefault: true,
			Source:    string(v.Default.Source),
		}
	}
	c.JSON(200, result)
}
func (h *BaseHandlers) changeError(c *gin.Context, err error) {
	status, code, message := 500, "changes_unavailable", "could not read changes"
	switch {
	case errors.Is(err, changes.ErrNotARepository):
		status, code, message = 409, "not_a_git_repository", "this folder is not a git repository"
	case errors.Is(err, changes.ErrBaseNotFound):
		status, code, message = 409, "base_not_found", "base ref not found"
	case errors.Is(err, changes.ErrStale):
		status, code, message = 409, "changes_stale", "changes moved since revision; refetch the summary"
	case errors.Is(err, changes.ErrTooLargeToExpand):
		status, code, message = 422, "too_large_to_expand", "file is too large to expand"
	case errors.Is(err, changes.ErrInvalidScope):
		status, code, message = 400, "invalid_scope", "invalid changes scope"
	case errors.Is(err, changes.ErrTooManyPaths):
		status, code, message = 400, "too_many_paths", "patch batch exceeds limits"
	default:
		if !errors.Is(err, changes.ErrUnavailable) {
			h.projectFileError(c, err)
			return
		}
	}
	h.respondSessionFilesError(c, status, code, errors.New(message))
}

func SessionChangesPayload(root projectfiles.Root, s changes.Summary) contract.SessionChangesResponse {
	files := make([]contract.FileChangePayload, 0, len(s.Files))
	for _, f := range s.Files {
		files = append(
			files,
			contract.FileChangePayload{
				Path:        f.Path,
				OldPath:     f.OldPath,
				Status:      string(f.Status),
				Additions:   f.Additions,
				Deletions:   f.Deletions,
				Binary:      f.Binary,
				Generated:   f.Generated,
				Sensitive:   f.Sensitive,
				Fingerprint: f.Fingerprint,
			},
		)
	}
	return contract.SessionChangesResponse{
		Scope: string(s.Scope),
		Root:  fileRootPayload(root),
		Head:  contract.ChangeHeadPayload{Branch: s.HeadBranch, SHA: s.HeadSHA},
		Base: contract.ChangeBasePayload{
			Ref:          s.BaseRef,
			Source:       string(s.BaseSource),
			MergeBaseSHA: s.MergeBaseSHA,
		},
		Totals: contract.ChangeTotalsPayload{
			Files:     s.Totals.Files,
			Additions: s.Totals.Additions,
			Deletions: s.Totals.Deletions,
			Partial:   s.Totals.Partial,
		},
		Files:     files,
		Truncated: s.Truncated,
		Revision:  s.Revision,
	}
}

func SessionChangesPullRequestPayload(
	view session.PullRequestView,
) (*contract.SessionPullRequestPayload, *contract.SessionPullRequestLookupPayload) {
	return pullRequestPayload(view)
}

func ChangePatchPayloads(patches []changes.Patch) []contract.ChangePatchPayload {
	result := make([]contract.ChangePatchPayload, 0, len(patches))
	for _, p := range patches {
		result = append(
			result,
			contract.ChangePatchPayload{
				Path:        p.Path,
				Patch:       p.Patch,
				TooLarge:    p.TooLarge,
				Sensitive:   p.Sensitive,
				Fingerprint: p.Fingerprint,
			},
		)
	}
	return result
}
