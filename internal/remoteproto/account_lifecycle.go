package remoteproto

import "time"

const (
	AccountTokenPrefix       = "cpzat_"
	AccountTokenIDPrefix     = "atk_"
	AccountTokenExchangePath = "/v1/account-tokens/exchange"
)

type AccountTokenExchange struct {
	InstallationID string `json:"installation_id"`
}

type ComputerRevalidation struct {
	InstallationID string `json:"installation_id,omitempty"`
}

type AccountTokenIdentity struct {
	AccountID string `json:"account_id"`
	Email     string `json:"email"`
	TokenID   string `json:"token_id"`
	TokenName string `json:"token_name"`
}

type AccountToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type AccountTokenList struct {
	Tokens []AccountToken `json:"tokens"`
}

type AccountTokenCreate struct {
	Name string `json:"name"`
}

type AccountTokenCreated struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}
