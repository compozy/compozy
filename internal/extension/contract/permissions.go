package contract

import (
	"slices"
	"strings"
)

// PermissionContract binds one Host API method to its operator-facing consent area.
type PermissionContract struct {
	Method     string
	Area       string
	Access     string
	Capability string
}

const (
	permissionAutomationRead  = "automation.read"
	permissionAutomationWrite = "automation.write"
	permissionHeartbeatRead   = "heartbeat.read"
	permissionHeartbeatWrite  = "heartbeat.write"
	permissionNetworkRead     = "network.read"
	permissionSessionRead     = "session.read"
	permissionSessionWrite    = "session.write"
	permissionSoulRead        = "soul.read"
	permissionSoulWrite       = "soul.write"
	permissionTaskRead        = "task.read"
	permissionTaskWrite       = "task.write"
)

var consentByHostAPIMethod = map[string]string{
	"automation/jobs":                permissionAutomationRead,
	"automation/jobs/get":            permissionAutomationRead,
	"automation/jobs/create":         permissionAutomationWrite,
	"automation/jobs/update":         permissionAutomationWrite,
	"automation/jobs/delete":         permissionAutomationWrite,
	"automation/jobs/trigger":        permissionAutomationWrite,
	"automation/jobs/runs":           permissionAutomationRead,
	"automation/triggers":            permissionAutomationRead,
	"automation/triggers/get":        permissionAutomationRead,
	"automation/triggers/create":     permissionAutomationWrite,
	"automation/triggers/update":     permissionAutomationWrite,
	"automation/triggers/delete":     permissionAutomationWrite,
	"automation/triggers/runs":       permissionAutomationRead,
	"automation/triggers/fire":       permissionAutomationWrite,
	"automation/runs":                permissionAutomationRead,
	"agents/heartbeat/delete":        permissionHeartbeatWrite,
	"agents/heartbeat/get":           permissionHeartbeatRead,
	"agents/heartbeat/history":       permissionHeartbeatRead,
	"agents/heartbeat/put":           permissionHeartbeatWrite,
	"agents/heartbeat/rollback":      permissionHeartbeatWrite,
	"agents/heartbeat/status":        permissionHeartbeatRead,
	"agents/heartbeat/validate":      permissionHeartbeatRead,
	"agents/heartbeat/wake":          permissionHeartbeatWrite,
	"agents/soul/delete":             permissionSoulWrite,
	"agents/soul/get":                permissionSoulRead,
	"agents/soul/history":            permissionSoulRead,
	"agents/soul/put":                permissionSoulWrite,
	"agents/soul/rollback":           permissionSoulWrite,
	"agents/soul/validate":           permissionSoulRead,
	"tasks":                          permissionTaskRead,
	"tasks/get":                      permissionTaskRead,
	"tasks/timeline":                 permissionTaskRead,
	"tasks/tree":                     permissionTaskRead,
	"tasks/dashboard":                permissionTaskRead,
	"tasks/inbox":                    permissionTaskRead,
	"tasks/create":                   permissionTaskWrite,
	"tasks/update":                   permissionTaskWrite,
	"tasks/cancel":                   permissionTaskWrite,
	"tasks/runs":                     permissionTaskRead,
	"tasks/runs/get":                 permissionTaskRead,
	"tasks/runs/enqueue":             permissionTaskWrite,
	"tasks/runs/start":               permissionTaskWrite,
	"tasks/runs/attach_session":      permissionTaskWrite,
	"tasks/runs/complete":            permissionTaskWrite,
	"tasks/runs/fail":                permissionTaskWrite,
	"tasks/runs/cancel":              permissionTaskWrite,
	"resources/list":                 "resources.read",
	"resources/get":                  "resources.read",
	"resources/snapshot":             "resources.write",
	"bridges/instances/list":         "bridge.read",
	"bridges/instances/get":          "bridge.read",
	"bridges/instances/report_state": "bridge.write",
	"bridges/messages/ingest":        "bridge.write",
	"memory/forget":                  "memory.write",
	"memory/recall":                  "memory.read",
	"memory/store":                   "memory.write",
	"models/list":                    "model.read",
	"models/refresh":                 "model.write",
	"models/status":                  "model.read",
	"network/status":                 permissionNetworkRead,
	"network/usage":                  permissionNetworkRead,
	"network/channels":               permissionNetworkRead,
	"network/peers":                  permissionNetworkRead,
	"network/threads":                permissionNetworkRead,
	"network/thread/get":             permissionNetworkRead,
	"network/thread/messages":        permissionNetworkRead,
	"network/directs":                permissionNetworkRead,
	"network/direct/resolve":         "network.write",
	"network/direct/messages":        permissionNetworkRead,
	"network/work/get":               permissionNetworkRead,
	"network/send":                   "network.write",
	"logs/list":                      "logs.read",
	"observe/health":                 "observe.read",
	"sandbox/list":                   "sandbox.read",
	"sandbox/info":                   "sandbox.read",
	"sandbox/exec":                   "sandbox.exec",
	"sessions/create":                permissionSessionWrite,
	"sessions/events":                permissionSessionRead,
	"sessions/health/get":            permissionSessionRead,
	"sessions/inputs/cancel":         permissionSessionWrite,
	"sessions/inputs/list":           permissionSessionRead,
	"sessions/inputs/promote":        permissionSessionWrite,
	"sessions/inputs/replace":        permissionSessionWrite,
	"sessions/list":                  permissionSessionRead,
	"sessions/prompt":                permissionSessionWrite,
	"sessions/runtime/clear":         permissionSessionWrite,
	"sessions/runtime/set":           permissionSessionWrite,
	"sessions/soul/refresh":          "soul.write",
	"sessions/status":                permissionSessionRead,
	"sessions/status/get":            permissionSessionRead,
	"sessions/stop":                  permissionSessionWrite,
	"sessions/archive":               permissionSessionWrite,
	"sessions/unarchive":             permissionSessionWrite,
	"skills/list":                    "skills.read",
	"clarify/ask":                    "clarify.ask",
	"view/patch":                     "view.write",
}

// PermissionContracts returns every Host API permission in protocol order.
func PermissionContracts() []PermissionContract {
	specs := HostAPIMethodSpecs()
	contracts := make([]PermissionContract, 0, len(specs))
	for _, spec := range specs {
		method := string(spec.Method)
		capability := consentByHostAPIMethod[method]
		area, access, _ := strings.Cut(capability, ".")
		if area == "session" {
			area = "sessions"
		}
		contracts = append(contracts, PermissionContract{
			Method:     method,
			Area:       area,
			Access:     access,
			Capability: capability,
		})
	}
	return contracts
}

// PermissionContractForMethod resolves one closed-set Host API permission.
func PermissionContractForMethod(method string) (PermissionContract, bool) {
	method = strings.TrimSpace(method)
	for _, contract := range PermissionContracts() {
		if contract.Method == method {
			if contract.Area == "" || contract.Access == "" || contract.Capability == "" {
				return PermissionContract{}, false
			}
			return contract, true
		}
	}
	return PermissionContract{}, false
}

// PermissionMethods returns the closed Host API permission set in stable order.
func PermissionMethods() []string {
	contracts := PermissionContracts()
	methods := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		methods = append(methods, contract.Method)
	}
	slices.Sort(methods)
	return methods
}
