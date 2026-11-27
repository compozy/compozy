-- name: GetComputerSSHSetup :one
SELECT * FROM computer_ssh_setups WHERE id = ? AND account_id = ?;

-- name: FindPendingComputerSSHSetup :one
SELECT * FROM computer_ssh_setups
WHERE account_id = ? AND host_key = ? AND step NOT IN ('done', 'cancelled');

-- name: CreateComputerSSHSetup :exec
INSERT INTO computer_ssh_setups (id, account_id, host_key, target_alias, name, kind, step, release_version, computer_id, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: WriteComputerSSHSetup :execrows
UPDATE computer_ssh_setups SET step = ?, platform = ?, binary_path = ?, service_warning = ?,
    reservation_id = ?, link_token_ref = ?, link_token_expires_at = ?, computer_id = ?,
    error_code = ?, error_detail = ?, updated_at = ?
WHERE id = ? AND account_id = ?;
