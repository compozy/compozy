// SPDX-License-Identifier: FSL-1.1-MIT

package admin

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/compozy/compozy/internal/cloud/store"
)

func (c *console) exportAccount(w http.ResponseWriter, r *http.Request) {
	ctx, account := r.Context(), r.PathValue("id")
	var content []byte
	var eventID int64
	err := c.db.Transaction(ctx, func(q *store.Queries) error {
		if _, err := q.LockAuthAccount(ctx, account); err != nil {
			return err
		}
		var err error
		eventID, err = q.AppendSecurityEventID(ctx, c.exportEvent(r, account, "account_exported", nil))
		if err != nil {
			return err
		}
		content, err = q.AdminAccountData(ctx, account, 0)
		return err
	})
	if err != nil {
		c.fail(w, err)
		return
	}
	c.download(w, eventID, "application/json", "compozy-account.json", content)
}

func (c *console) exportContacts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var content []byte
	var eventID int64
	err := c.db.Transaction(ctx, func(q *store.Queries) error {
		contacts, err := q.AdminOptedInContacts(ctx)
		if err != nil {
			return err
		}
		content, err = contactsCSV(contacts)
		if err != nil {
			return err
		}
		eventID, err = q.AppendSecurityEventID(ctx,
			c.exportEvent(r, "", "contacts_exported", map[string]any{"count": len(contacts)}))
		return err
	})
	if err != nil {
		c.fail(w, err)
		return
	}
	c.download(w, eventID, "text/csv; charset=utf-8", "compozy-contacts.csv", content)
}

func (c *console) exportEvent(r *http.Request, account, kind string, details map[string]any) store.SecurityEvent {
	return store.SecurityEvent{
		AccountID: account, Type: kind, Actor: "admin:" + operator(r), At: c.config.Now(), Details: details,
	}
}

func (c *console) download(w http.ResponseWriter, event int64, mediaType, filename string, content []byte) {
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("X-Compozy-Security-Event", strconv.FormatInt(event, 10))
	w.Header().Set("Link", "<"+eventURL(event)+">; rel=\"related\"")
	if _, err := w.Write(content); err != nil {
		c.config.Logger.Debug("Cloud admin download interrupted", "event_id", event)
	}
}

func contactsCSV(contacts []store.AdminContact) ([]byte, error) {
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.Write([]string{"email", "name", "consent_at", "consent_source"}); err != nil {
		return nil, err
	}
	for _, contact := range contacts {
		if err := writer.Write([]string{
			csvText(
				contact.Email,
			),
			csvText(contact.Name),
			contact.ConsentedAt.UTC().Format(time.RFC3339Nano),
			contact.Source,
		}); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return out.Bytes(), writer.Error()
}

func csvText(s string) string {
	for _, first := range strings.TrimLeftFunc(s, unicode.IsSpace) {
		if strings.ContainsRune("=+-@", first) {
			return "'" + s
		}
		break
	}
	return s
}
