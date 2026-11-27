-- SPDX-License-Identifier: FSL-1.1-MIT

-- +goose Up
ALTER TABLE tunnels ADD COLUMN admin_takedown_at timestamptz;

-- The account-token schema is owned by the sign-in slice. Consume its safe metadata when present.
-- +goose StatementBegin
CREATE FUNCTION admin_account_token_data(owner_id text) RETURNS jsonb LANGUAGE plpgsql STABLE AS $$
BEGIN
    IF to_regclass('account_tokens') IS NULL OR to_regclass('account_token_instances') IS NULL THEN
        RETURN '{"tokens": [], "installations": []}'::jsonb;
    END IF;
    RETURN jsonb_build_object(
        'tokens', coalesce((
            SELECT jsonb_agg(jsonb_build_object(
                'id', t.id, 'name', t.name, 'created_at', t.created_at, 'last_used_at', t.last_used_at, 'revoked_at', t.revoked_at
            ) ORDER BY t.id)
            FROM account_tokens t WHERE t.account_id = owner_id
        ), '[]'),
        'installations', coalesce((
            SELECT jsonb_agg(jsonb_build_object(
                'id', i.installation_id, 'source', 'token', 'token_id', i.token_id, 'last_seen_at', i.last_seen_at
            ) ORDER BY i.installation_id, i.token_id)
            FROM account_token_instances i JOIN account_tokens t ON t.id = i.token_id WHERE t.account_id = owner_id
        ), '[]')
    );
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP FUNCTION admin_account_token_data(text);
ALTER TABLE tunnels DROP COLUMN admin_takedown_at;
