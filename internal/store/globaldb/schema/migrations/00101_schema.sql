-- +goose Up
CREATE VIEW `loop_generation_output_payloads` AS
SELECT loop_run_id,
       generation,
       node_id,
       item_index,
       CAST(COALESCE(json_extract(output_ref, '$.payload_ref'), '') AS TEXT) AS payload_ref,
       CAST(json_extract(json_extract(output_ref, '$.payload_ref'), '$.kind') AS TEXT) AS payload_kind
FROM loop_generation_outputs
WHERE json_valid(output_ref);

-- +goose Down
DROP VIEW `loop_generation_output_payloads`;

