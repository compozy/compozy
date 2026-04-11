package extensions

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestCapabilityCheckerAllowsGrantedMethodsAndHelpers(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		accepted []Capability
		method   string
	}{
		{
			name:     "granted host api capability",
			accepted: []Capability{CapabilityTasksRead},
			method:   "host.tasks.get",
		},
		{
			name:     "helper with no capability requirement",
			accepted: nil,
			method:   "host.prompts.render",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			checker := NewCapabilityChecker(tc.accepted)
			if err := checker.CheckHostMethod(tc.method); err != nil {
				t.Fatalf("CheckHostMethod(%q) error = %v, want nil", tc.method, err)
			}
		})
	}
}

func TestCapabilityCheckerReturnsDeniedErrorForMissingGrant(t *testing.T) {
	t.Parallel()

	checker := NewCapabilityChecker([]Capability{CapabilityTasksRead})

	err := checker.CheckHostMethod("host.tasks.create")
	if err == nil {
		t.Fatal("CheckHostMethod() error = nil, want denial")
	}

	var denied *CapabilityDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("CheckHostMethod() error = %T, want *CapabilityDeniedError", err)
	}
	if !reflect.DeepEqual(denied.Missing, []Capability{CapabilityTasksCreate}) {
		t.Fatalf("Missing = %v, want [%q]", denied.Missing, CapabilityTasksCreate)
	}
	if !reflect.DeepEqual(denied.Granted, []Capability{CapabilityTasksRead}) {
		t.Fatalf("Granted = %v, want [%q]", denied.Granted, CapabilityTasksRead)
	}
}

func TestCapabilityCheckerReturnsDeniedErrorForMultipleMissingCapabilities(t *testing.T) {
	t.Parallel()

	checker := NewCapabilityChecker([]Capability{CapabilityTasksRead})

	err := checker.Check(
		"compound.target",
		CapabilityTasksCreate,
		CapabilityMemoryWrite,
		CapabilityTasksCreate,
	)
	if err == nil {
		t.Fatal("Check() error = nil, want denial")
	}

	var denied *CapabilityDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("Check() error = %T, want *CapabilityDeniedError", err)
	}

	wantMissing := []Capability{CapabilityMemoryWrite, CapabilityTasksCreate}
	if !reflect.DeepEqual(denied.Missing, wantMissing) {
		t.Fatalf("Missing = %v, want %v", denied.Missing, wantMissing)
	}
	if !reflect.DeepEqual(denied.Granted, []Capability{CapabilityTasksRead}) {
		t.Fatalf("Granted = %v, want [%q]", denied.Granted, CapabilityTasksRead)
	}
}

func TestCapabilityCheckerMapsHookFamiliesToCapabilities(t *testing.T) {
	t.Parallel()

	checker := NewCapabilityChecker([]Capability{CapabilityTasksRead})

	err := checker.CheckHook(HookPromptPostBuild)
	if err == nil {
		t.Fatal("CheckHook() error = nil, want denial")
	}

	var denied *CapabilityDeniedError
	if !errors.As(err, &denied) {
		t.Fatalf("CheckHook() error = %T, want *CapabilityDeniedError", err)
	}
	if !reflect.DeepEqual(denied.Missing, []Capability{CapabilityPromptMutate}) {
		t.Fatalf("Missing = %v, want [%q]", denied.Missing, CapabilityPromptMutate)
	}
	if denied.Method != string(HookPromptPostBuild) {
		t.Fatalf("Method = %q, want %q", denied.Method, HookPromptPostBuild)
	}
}

func TestCapabilityDeniedErrorSerializesJSONRPCPayload(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(&CapabilityDeniedError{
		Method:  "host.tasks.create",
		Missing: []Capability{CapabilityTasksCreate},
		Granted: []Capability{CapabilityTasksRead},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Method   string       `json:"method"`
			Required []Capability `json:"required"`
			Granted  []Capability `json:"granted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Code != capabilityDeniedCode {
		t.Fatalf("Code = %d, want %d", decoded.Code, capabilityDeniedCode)
	}
	if decoded.Message != capabilityDeniedMessage {
		t.Fatalf("Message = %q, want %q", decoded.Message, capabilityDeniedMessage)
	}
	if decoded.Data.Method != "host.tasks.create" {
		t.Fatalf("Data.Method = %q, want host.tasks.create", decoded.Data.Method)
	}
	if !reflect.DeepEqual(decoded.Data.Required, []Capability{CapabilityTasksCreate}) {
		t.Fatalf("Data.Required = %v, want [%q]", decoded.Data.Required, CapabilityTasksCreate)
	}
	if !reflect.DeepEqual(decoded.Data.Granted, []Capability{CapabilityTasksRead}) {
		t.Fatalf("Data.Granted = %v, want [%q]", decoded.Data.Granted, CapabilityTasksRead)
	}
}

func TestCapabilityDeniedErrorRequestErrorAndString(t *testing.T) {
	t.Parallel()

	denied := &CapabilityDeniedError{
		Method:  "host.tasks.create",
		Missing: []Capability{CapabilityTasksCreate},
		Granted: []Capability{CapabilityTasksRead},
	}

	requestErr := denied.RequestError()
	if requestErr == nil {
		t.Fatal("RequestError() = nil, want payload")
	}
	if requestErr.Code != capabilityDeniedCode {
		t.Fatalf("RequestError().Code = %d, want %d", requestErr.Code, capabilityDeniedCode)
	}
	if requestErr.Message != capabilityDeniedMessage {
		t.Fatalf("RequestError().Message = %q, want %q", requestErr.Message, capabilityDeniedMessage)
	}
	if !strings.Contains(denied.Error(), "host.tasks.create") {
		t.Fatalf("Error() = %q, want method text", denied.Error())
	}
}

func TestCapabilityCheckerRejectsUnknownTargets(t *testing.T) {
	t.Parallel()

	checker := NewCapabilityChecker(nil)

	testCases := []struct {
		name string
		run  func() error
		want string
	}{
		{
			name: "unknown host method",
			run: func() error {
				return checker.CheckHostMethod("host.tasks.unknown")
			},
			want: `unknown capability method "host.tasks.unknown"`,
		},
		{
			name: "unknown hook",
			run: func() error {
				return checker.CheckHook(HookName("hook.unknown"))
			},
			want: `unknown capability hook "hook.unknown"`,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.run()
			if err == nil {
				t.Fatal("error = nil, want unknown target failure")
			}

			var unknownErr *UnknownCapabilityTargetError
			if !errors.As(err, &unknownErr) {
				t.Fatalf("error = %T, want *UnknownCapabilityTargetError", err)
			}
			if unknownErr.Error() != tc.want {
				t.Fatalf("Error() = %q, want %q", unknownErr.Error(), tc.want)
			}
		})
	}
}

func TestWarnCapabilityDeniedEmitsStructuredWarning(t *testing.T) {
	logBuf := captureDefaultLogger(t)

	checker := NewCapabilityChecker([]Capability{CapabilityTasksRead})
	err := checker.CheckHostMethod("host.tasks.create")
	if err == nil {
		t.Fatal("CheckHostMethod() error = nil, want denial")
	}

	WarnCapabilityDenied("extension.host_api", "example-ext", err)

	records := decodeLogRecords(t, logBuf)
	if len(records) != 1 {
		t.Fatalf("len(log records) = %d, want 1", len(records))
	}
	if got := records[0]["msg"]; got != "extension capability denied" {
		t.Fatalf("msg = %v, want capability denial warning", got)
	}
	if got := records[0]["component"]; got != "extension.host_api" {
		t.Fatalf("component = %v, want extension.host_api", got)
	}
	if got := records[0]["action"]; got != "capability_denied" {
		t.Fatalf("action = %v, want capability_denied", got)
	}
	if got := records[0]["extension"]; got != "example-ext" {
		t.Fatalf("extension = %v, want example-ext", got)
	}
	if got := records[0]["method"]; got != "host.tasks.create" {
		t.Fatalf("method = %v, want host.tasks.create", got)
	}

	if got := anySliceToStrings(t, records[0]["missing"]); !reflect.DeepEqual(got, []string{"tasks.create"}) {
		t.Fatalf("missing = %v, want [tasks.create]", got)
	}
	if got := anySliceToStrings(t, records[0]["granted"]); !reflect.DeepEqual(got, []string{"tasks.read"}) {
		t.Fatalf("granted = %v, want [tasks.read]", got)
	}
}

func anySliceToStrings(t *testing.T, value any) []string {
	t.Helper()

	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value = %T, want []any", value)
	}

	values := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("slice item = %T, want string", item)
		}
		values = append(values, text)
	}
	return values
}
