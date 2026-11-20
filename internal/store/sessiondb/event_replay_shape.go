package sessiondb

// The canonical event codec puts message text and tool identity in the envelope.
// Keep their structure for counting, without returning tool inputs or outputs.
const sessionReplayShapeColumns = `id, sequence, turn_id, type, agent_name,
CASE
  WHEN type IN ('tool_call', 'tool_result') THEN CASE WHEN json_valid(content) THEN json_object(
    'schema', json_extract(content, '$.schema'),
    'type', json_extract(content, '$.type'),
    'sessionUpdate', json_extract(content, '$.sessionUpdate'),
    'status', json_extract(content, '$.status'),
    'tool_call_id', json_extract(content, '$.tool_call_id'),
    'toolCallId', json_extract(content, '$.toolCallId')
  ) ELSE '' END
  WHEN type IN ('user_message', 'synthetic_reentry', 'agent_message', 'thought') THEN CASE
    WHEN json_valid(content) THEN CASE
      WHEN json_extract(content, '$.schema') = 'compozy.session.event.v1' THEN json_object(
        'schema', 'compozy.session.event.v1',
        'type', json_extract(content, '$.type'),
        'text', ` + replayTextShape + `,
        'attachments', json(CASE WHEN json_array_length(content, '$.attachments') > 0 THEN '[{}]' ELSE '[]' END)
      )
      ELSE content END
    ELSE ` + replayPlainTextShape + ` END
  WHEN type GLOB 'transcript_marker*' THEN content
  ELSE ''
END AS content, archived, timestamp`

const replayWhitespace = `char(9,10,11,12,13,32,133,160,5760,8192,8193,8194,8195,8196,8197,8198,8199,8200,8201,8202,8232,8233,8239,8287,12288)`

const replayText = `COALESCE(NULLIF(content ->> '$.authored_text', ''), content ->> '$.text', '')`

const replayTextShape = `CASE WHEN trim(` + replayText + `, ` + replayWhitespace + `) = '' THEN '' ELSE 'x' END`

const replayPlainTextShape = `CASE WHEN trim(content, ` + replayWhitespace + `) = '' THEN '' ELSE 'x' END`
