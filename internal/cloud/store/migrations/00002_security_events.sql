-- +goose Up
CREATE TABLE security_events (
    id bigserial PRIMARY KEY,
    account_id text,
    type text NOT NULL CHECK (type <> ''),
    actor text NOT NULL CHECK (actor = 'system' OR actor ~ '^(account|admin):.+$'),
    at timestamptz NOT NULL,
    details_json jsonb NOT NULL CHECK (jsonb_typeof(details_json) = 'object')
);
CREATE INDEX security_events_account_idx ON security_events (account_id, at DESC);
CREATE INDEX security_events_at_idx ON security_events (at);

CREATE TABLE security_event_counts (
    type text PRIMARY KEY,
    count bigint NOT NULL CHECK (count > 0)
);

-- +goose StatementBegin
CREATE FUNCTION protect_security_events() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' AND pg_trigger_depth() = 2
       AND current_setting('compozy.security_event_retention', true) = 'on' THEN
        RETURN OLD;
    END IF;
    RAISE EXCEPTION 'security_events is append-only' USING ERRCODE = '55000';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER security_events_immutable BEFORE UPDATE OR DELETE ON security_events
    FOR EACH ROW EXECUTE FUNCTION protect_security_events();
CREATE TRIGGER security_events_no_truncate BEFORE TRUNCATE ON security_events
    FOR EACH STATEMENT EXECUTE FUNCTION protect_security_events();

-- Retention enters through a nested trigger so direct log DML cannot use the retention marker.
CREATE VIEW security_event_retention AS SELECT NULL::timestamptz AS cutoff WHERE false;

-- +goose StatementBegin
CREATE FUNCTION compact_security_events() RETURNS trigger LANGUAGE plpgsql
SET compozy.security_event_retention TO 'on'
AS $$
BEGIN
    WITH retired AS (
        DELETE FROM security_events WHERE at < NEW.cutoff RETURNING type
    )
    INSERT INTO security_event_counts (type, count)
        SELECT type, count(*) FROM retired GROUP BY type ORDER BY type
        ON CONFLICT (type) DO UPDATE SET count = security_event_counts.count + excluded.count;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER security_event_retention_job INSTEAD OF INSERT ON security_event_retention
    FOR EACH ROW EXECUTE FUNCTION compact_security_events();
