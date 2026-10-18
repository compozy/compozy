package contract

import "time"

type AppTokenPayload struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Origins    []string   `json:"origins"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type CreateAppTokenRequest struct {
	Name    string   `json:"name"`
	Origins []string `json:"origins,omitempty"`
}

type UpdateAppTokenRequest struct {
	Name    *string   `json:"name,omitempty"`
	Origins *[]string `json:"origins,omitempty"`
}

type AppTokenIssuedResponse struct {
	AppToken AppTokenPayload `json:"app_token"`
	Token    string          `json:"token"`
}

type AppTokenResponse struct {
	AppToken AppTokenPayload `json:"app_token"`
}
type AppTokensResponse struct {
	AppTokens []AppTokenPayload `json:"app_tokens"`
}
type AppTokenRevokeResponse struct {
	AppToken AppTokenPayload `json:"app_token"`
	Changed  bool            `json:"changed"`
	Canceled int             `json:"canceled"`
}
