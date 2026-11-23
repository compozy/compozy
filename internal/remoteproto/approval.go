package remoteproto

import (
	"crypto"
	"encoding/json/v2"
	"errors"
	"time"
)

const (
	approvalDecisionType = "compozy-approval+jwt"
	ApprovalTTL          = 10 * time.Minute
)

var ErrApprovalSignature = errors.New("approval_signature_invalid")

type ApprovalDecision struct {
	ApprovalID string     `json:"approval_id"`
	ComputerID ComputerID `json:"computer_id"`
	DeviceID   DeviceID   `json:"device_id"`
	JKT        string     `json:"jkt"`
	Decision   string     `json:"decision"`
	IssuedAt   int64      `json:"iat"`
	JTI        string     `json:"jti"`
}

func SignApprovalDecision(signer crypto.Signer, decision ApprovalDecision) (string, error) {
	if signer == nil {
		return "", ErrApprovalSignature
	}
	key, err := NewPublicJWK(signer.Public())
	if err != nil || key.Kty != keyEC {
		return "", ErrApprovalSignature
	}
	return signDeviceJWS(signer, key, approvalDecisionType, decision)
}

func VerifyApprovalDecision(token string, now time.Time) (ApprovalDecision, string, error) {
	jws, err := parseCompact(token)
	if err != nil || jws.header.JWK == nil || len(token) > maxProofSize {
		return ApprovalDecision{}, "", ErrApprovalSignature
	}
	jkt, err := jws.header.JWK.Thumbprint()
	if err != nil || !verifyDeviceSignature(jws, approvalDecisionType, jkt) {
		return ApprovalDecision{}, "", ErrApprovalSignature
	}
	var decision ApprovalDecision
	if err := json.Unmarshal(jws.body, &decision); err != nil || !decision.validAt(now) {
		return ApprovalDecision{}, "", ErrApprovalSignature
	}
	return decision, jkt, nil
}

func (d ApprovalDecision) validAt(now time.Time) bool {
	_, err := decodeCoordinate(d.JKT)
	return err == nil && d.ApprovalID != "" && d.ComputerID != "" && d.DeviceID != "" && d.JTI != "" &&
		len(d.JTI) <= 128 && (d.Decision == "approve" || d.Decision == "deny") &&
		d.IssuedAt >= now.Add(-ApprovalTTL).Unix() && d.IssuedAt <= now.Add(GrantLeeway).Unix()
}
