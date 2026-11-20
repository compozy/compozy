package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/session"
)

func sideErrorCode(err error) string {
	for _, item := range []struct {
		err  error
		code string
	}{
		{session.ErrSideMessageRequired, "side_message_required"},
		{session.ErrSideAttachmentsUnsupported, "side_attachments_unsupported"},
		{session.ErrSideSourceEmpty, "session_side_source_empty"},
		{session.ErrSideNested, "session_side_nested"},
		{session.ErrSideUnsupported, "session_side_unsupported"},
		{session.ErrSideParentArchived, "session_side_parent_archived"},
		{session.ErrSideNotDerivable, "session_side_not_derivable"},
		{session.ErrSideFixedParent, "session_side_fixed_parent"},
		{session.ErrSideWorktreeFixed, "session_side_worktree_fixed"},
		{session.ErrSideLaterUnsupported, "session_side_later_unsupported"},
		{session.ErrSideGoalUnsupported, "session_side_goal_unsupported"},
		{session.ErrSideSpawnUnsupported, "session_side_spawn_unsupported"},
	} {
		if errors.Is(err, item.err) {
			return item.code
		}
	}
	return ""
}

func statusForSideError(err error) (int, bool) {
	code := sideErrorCode(err)
	if code == "" {
		return 0, false
	}
	if code == "side_message_required" || code == "side_attachments_unsupported" {
		return http.StatusBadRequest, true
	}
	return http.StatusConflict, true
}
