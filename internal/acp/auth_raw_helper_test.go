package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Raw ACP helper scenarios exercise authentication wire ordering with full
// control over the initialize authMethods discriminator, which the pinned SDK
// union cannot express (unknown types must fail closed, never decode as agent
// methods). The scenario is the TestACPHelperProcess scenario env value after
// the "auth_raw_" prefix; os.Getenv(testHelperFileKey) is the record file.

type rawAuthEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

type rawAuthHelper struct {
	scenario string
	record   *os.File
	stdout   *json.Encoder
	reader   *bufio.Reader
	sessions int
	authed   int
	peerSeq  int
}

func runRawAuthHelperProcess(scenario, recordPath, capturePath string) {
	var record *os.File
	if strings.TrimSpace(recordPath) != "" {
		var err error
		record, err = os.Create(recordPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create record file: %v\n", err)
			os.Exit(1)
		}
		defer record.Close()
	}
	var captureFile *os.File
	if strings.TrimSpace(capturePath) != "" {
		var err error
		captureFile, err = os.Create(capturePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create capture file: %v\n", err)
			os.Exit(1)
		}
		defer captureFile.Close()
	}
	input := bufio.NewReader(os.Stdin)
	helper := &rawAuthHelper{
		scenario: strings.TrimPrefix(scenario, "auth_raw_"),
		record:   record,
		stdout:   json.NewEncoder(os.Stdout),
		reader:   input,
	}
	helper.note("home HOME=" + os.Getenv("HOME"))
	for key, value := range helper.markers() {
		helper.note("env " + key + "=" + value)
	}
	// Live login progress rides stderr to the operator's writer; stdout stays
	// JSON-RPC frames. The marker proves the sink wiring without secrets.
	fmt.Fprintln(os.Stderr, "auth peer starting "+helper.scenario+" tok-fake-secret-0")
	helper.note(fmt.Sprintf("pid %d", os.Getpid()))
	helper.serve(captureFile)
	helper.note("exit")
	os.Exit(0)
}

func (h *rawAuthHelper) markers() map[string]string {
	if value, ok := os.LookupEnv("COMPOZY_TEST_AUTH_MARKER"); ok {
		return map[string]string{"COMPOZY_TEST_AUTH_MARKER": value}
	}
	return map[string]string{}
}

func (h *rawAuthHelper) note(line string) {
	if h.record == nil {
		return
	}
	fmt.Fprintln(h.record, line)
}

func (h *rawAuthHelper) serve(capture *os.File) {
	for {
		line, err := h.reader.ReadBytes('\n')
		if err != nil {
			return
		}
		if capture != nil {
			if _, err := capture.Write(line); err != nil {
				return
			}
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		var frame rawAuthEnvelope
		if err := json.Unmarshal([]byte(trimmed), &frame); err != nil {
			continue
		}
		if len(frame.ID) == 0 {
			continue // notification: nothing to answer
		}
		h.note("request " + frame.Method)
		h.dispatch(frame)
	}
}

func (h *rawAuthHelper) respond(id json.RawMessage, result any) {
	h.stdout.Encode(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
}

func (h *rawAuthHelper) respondError(id json.RawMessage, code int, message string) {
	h.stdout.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error":   map[string]any{"code": code, "message": message},
	})
}

func (h *rawAuthHelper) dispatch(frame rawAuthEnvelope) {
	switch frame.Method {
	case "initialize":
		h.respond(frame.ID, h.initializeResult())
	case "session/new":
		h.sessions++
		h.handleSessionNew(frame)
	case "authenticate":
		h.authed++
		h.handleAuthenticate(frame)
	case "session/close":
		h.respond(frame.ID, map[string]any{})
	default:
		h.respondError(frame.ID, -32601, "method not found: "+frame.Method)
	}
}

func (h *rawAuthHelper) initializeResult() map[string]any {
	caps := map[string]any{
		"loadSession":        false,
		"promptCapabilities": map[string]any{},
	}
	if h.scenario != "login_no_close" {
		caps["sessionCapabilities"] = map[string]any{"close": map[string]any{}}
	}
	return map[string]any{
		"protocolVersion":   1,
		"agentCapabilities": caps,
		"authMethods":       h.authMethods(),
	}
}

func (h *rawAuthHelper) authMethods() []map[string]any {
	other := map[string]any{"id": "other-method", "name": "Other"}
	switch h.scenario {
	case "method_absent":
		return []map[string]any{other}
	case "method_terminal":
		return []map[string]any{
			{"id": "oauth-personal", "name": "Personal", "type": "terminal"},
			other,
		}
	case "method_unknown":
		return []map[string]any{
			{"id": "oauth-personal", "name": "Personal", "type": "custom-unknown"},
			other,
		}
	case "method_agent_type":
		return []map[string]any{
			{"id": "oauth-personal", "name": "Personal", "type": "agent"},
			other,
		}
	case "method_env_var":
		return []map[string]any{
			{
				"id":   "oauth-personal",
				"name": "Personal",
				"type": "env_var",
				"vars": []map[string]any{{"name": "TOKEN", "description": "token"}},
			},
			other,
		}
	default:
		return []map[string]any{
			{"id": "oauth-personal", "name": "Personal"},
			other,
		}
	}
}

func (h *rawAuthHelper) handleSessionNew(frame rawAuthEnvelope) {
	switch h.scenario {
	case "login_ready":
		h.respond(frame.ID, map[string]any{"sessionId": "sess-auth"})
	case "login_clean", "login_no_close", "login_refuse", "method_agent_type":
		if h.sessions == 1 {
			h.respondError(frame.ID, -32000, "authentication required")
			return
		}
		h.respond(frame.ID, map[string]any{"sessionId": "sess-auth"})
	default:
		h.respondError(frame.ID, -32000, "authentication required")
	}
}

func (h *rawAuthHelper) handleAuthenticate(frame rawAuthEnvelope) {
	var params struct {
		MethodID string `json:"methodId"`
	}
	if err := json.Unmarshal(frame.Params, &params); err != nil {
		h.note("authenticate params error")
		return
	}
	h.note("authenticate methodId=" + params.MethodID)
	switch h.scenario {
	case "login_block":
		// Never answer: caller cancellation or timeout must settle the attempt
		// and still join process cleanup.
		select {}
	case "login_refuse":
		h.probeInboundWork()
		h.respond(frame.ID, map[string]any{})
	default:
		h.respond(frame.ID, map[string]any{})
	}
}

// probeInboundWork asks the client for filesystem, terminal, and permission
// work. Every answer must be a refusal; the write target must stay absent.
func (h *rawAuthHelper) probeInboundWork() {
	artifact := ""
	if h.record != nil {
		artifact = filepath.Join(filepath.Dir(h.record.Name()), "acp-auth-refuse-probe")
	} else {
		artifact = filepath.Join(os.TempDir(), "acp-auth-refuse-probe")
	}
	h.note("artifact " + artifact)
	h.ask("fs/read_text_file", map[string]any{"sessionId": "sess-unbound", "path": artifact})
	h.ask("fs/write_text_file", map[string]any{
		"sessionId": "sess-unbound",
		"path":      artifact,
		"content":   "refused",
	})
	h.ask("terminal/create", map[string]any{"command": "touch", "args": []string{artifact}})
	h.ask("session/request_permission", map[string]any{
		"sessionId": "sess-unbound",
		"toolCall":  map[string]any{"toolCallId": "tc-probe", "title": "probe"},
		"options": []map[string]any{
			{"optionId": "allow-once", "name": "Allow once", "kind": "allow_once"},
			{"optionId": "reject-once", "name": "Reject once", "kind": "reject_once"},
		},
	})
}

func (h *rawAuthHelper) ask(method string, params map[string]any) {
	h.peerSeq++
	id := fmt.Sprintf("peer-%d", h.peerSeq)
	h.stdout.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	for {
		line, err := h.reader.ReadBytes('\n')
		if err != nil {
			h.note("inbound " + method + " transport-closed")
			return
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		var frame rawAuthEnvelope
		if err := json.Unmarshal([]byte(trimmed), &frame); err != nil {
			continue
		}
		var gotID string
		if err := json.Unmarshal(frame.ID, &gotID); err != nil {
			continue
		}
		if gotID != id {
			// A concurrent client notification or unrelated frame: keep waiting
			// for the refusal answer to this request.
			continue
		}
		if len(frame.Error) > 0 {
			var errObj struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(frame.Error, &errObj); err != nil {
				h.note("inbound " + method + " error=parse")
				return
			}
			h.note(fmt.Sprintf("inbound %s error=%d", method, errObj.Code))
			return
		}
		compact, marshalErr := json.Marshal(frame.Result)
		if marshalErr != nil {
			h.note("inbound " + method + " error=marshal")
			return
		}
		h.note("inbound " + method + " result=" + string(compact))
		return
	}
}
