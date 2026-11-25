// SPDX-License-Identifier: FSL-1.1-MIT

package email

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
)

type Broadcast struct {
	ID, Subject, Text string
}

func (r *Resend) ValidateBroadcast(broadcast Broadcast) error {
	if r == nil || !r.SyncsContacts() || r.config.PostalAddress == "" {
		return errors.New("product news requires a Resend news segment, contacts key and a postal address")
	}
	if strings.TrimSpace(broadcast.ID) == "" || strings.TrimSpace(broadcast.Subject) == "" ||
		strings.TrimSpace(broadcast.Text) == "" {
		return errors.New("a broadcast needs an ID, subject and message")
	}
	return nil
}

// SendBroadcast delivers to one recipient so both unsubscribe headers contain that account's signed URL.
func (r *Resend) SendBroadcast(
	ctx context.Context, broadcast Broadcast, accountID, address string, links *UnsubscribeLinks,
) error {
	if err := r.ValidateBroadcast(broadcast); err != nil {
		return err
	}
	unsubscribe := links.URL(accountID)
	key := fmt.Sprintf("compozy-broadcast/%x", sha256.Sum256([]byte(broadcast.ID+"\x00"+accountID)))
	return r.send(ctx, key, Message{
		To: address, Subject: broadcast.Subject,
		Text: strings.TrimRight(broadcast.Text, "\r\n") + "\n\nCompozy, LLC\n" + r.config.PostalAddress +
			"\nUnsubscribe: " + unsubscribe + "\n",
	}, map[string]string{
		"List-Unsubscribe":      "<" + unsubscribe + ">",
		"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
	})
}
