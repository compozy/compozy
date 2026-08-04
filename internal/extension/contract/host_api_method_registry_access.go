package contract

const (
	hostAPISessionsPromptParamsValue = "SessionsPromptParams"
	hostAPISessionPromptResultValue  = "SessionPromptResult"
	hostAPISessionStatusValue        = "SessionStatus"
)

// HostAPIMethodSpecs returns the canonical Host API method registry in wire order.
func HostAPIMethodSpecs() []HostAPIMethodSpec {
	return append(append([]HostAPIMethodSpec(nil), hostAPIMethodSpecs...), clarifyHostAPIMethodSpec())
}
