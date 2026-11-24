// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/cloud/email"
)

func sendPrivacyReply(ctx context.Context, requestID, path string, getenv func(string) string) (retErr error) {
	if !signingKeyID.MatchString(requestID) {
		return errors.New("invalid privacy request ID")
	}
	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return err
	}
	resend, _, err := loadResend(getenv, environment)
	if err != nil {
		return err
	}
	if resend == nil {
		return errors.New("Resend is required for privacy replies")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.New("open private reply file")
	}
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	var reply struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.UnmarshalRead(io.LimitReader(file, 64<<10), &reply); err != nil {
		return errors.New("privacy reply file must contain a recipient and plain text")
	}
	address, err := mail.ParseAddress(reply.To)
	if err != nil || strings.TrimSpace(reply.Text) == "" {
		return errors.New("privacy reply requires a valid email address and message")
	}
	key := fmt.Sprintf("privacy/%x", sha256.Sum256([]byte(requestID)))
	return resend.Send(ctx, key, email.Message{
		To: address.Address, Subject: "Your Compozy privacy request", Text: reply.Text,
	})
}
