-- +goose Up
-- rebuild "idx_tool_approval_pending_recovery" so resume_fence precedes expires_at
DROP INDEX `idx_tool_approval_pending_recovery`;
CREATE INDEX `idx_tool_approval_pending_recovery` ON `tool_approval_pending` (`approval_status`, `execution_status`, `resume_fence`, `expires_at`);
