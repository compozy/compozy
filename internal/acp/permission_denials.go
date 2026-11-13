package acp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	acpsdk "github.com/coder/acp-go-sdk"
)

// ErrPermissionExpired distinguishes an unanswered approval from an operator rejection.
var ErrPermissionExpired = errors.New("acp: approval expired; ask the operator to prompt the session before retrying")

// PermissionDenials outlives an ACP process and is cleared by the next human prompt.
type PermissionDenials struct {
	mu      sync.Mutex
	expired map[string]string
	epoch   uint64
}

func (d *PermissionDenials) Contains(key string) bool {
	if d == nil || key == "" {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	_, found := d.expired[key]
	return found
}

func (d *PermissionDenials) Record(key string, epoch uint64) {
	d.record(key, epoch, "timeout")
}

func (d *PermissionDenials) record(key string, epoch uint64, actor string) {
	if d == nil || key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.epoch != epoch {
		return
	}
	if d.expired == nil {
		d.expired = make(map[string]string)
	}
	d.expired[key] = actor
}

func (d *PermissionDenials) Clear() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	clear(d.expired)
	d.epoch++
}

func permissionRequestDigest(request acpsdk.RequestPermissionRequest) string {
	if request.Meta != nil {
		if digest, ok := request.Meta["compozy_input_digest"].(string); ok && digest != "" {
			return permissionToolID(request) + ":" + digest
		}
	}
	data, err := json.Marshal(struct {
		Kind      *acpsdk.ToolKind
		Title     *string
		Locations []acpsdk.ToolCallLocation
		Input     any
	}{request.ToolCall.Kind, request.ToolCall.Title, request.ToolCall.Locations, request.ToolCall.RawInput})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (d *PermissionDenials) Epoch() uint64 {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.epoch
}

func (d *PermissionDenials) timedOut(key string) bool {
	if d == nil {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.expired[key] == "timeout"
}
