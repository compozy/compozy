// SPDX-License-Identifier: FSL-1.1-MIT

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

func sendBroadcast(ctx context.Context, args []string, getenv func(string) string, out io.Writer) error {
	var news email.Broadcast
	flags := flag.NewFlagSet("broadcast send", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.StringVar(&news.ID, "id", "", "Stable identifier for this broadcast")
	flags.StringVar(&news.Subject, "subject", "", "Email subject")
	file := flags.String("text-file", "", "Path to the plain text message")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *file == "" {
		return errors.New("usage: compozy-cloud broadcast send --id ID --subject SUBJECT --text-file PATH")
	}
	body, err := readBroadcastText(*file)
	if err != nil {
		return err
	}
	news.Text = body

	environment, err := cloudEnvironment(getenv)
	if err != nil {
		return err
	}
	resend, key, err := loadResend(getenv, environment)
	if err != nil {
		return err
	}
	if err := resend.ValidateBroadcast(news); err != nil {
		return err
	}
	issuer := remoteproto.CloudIssuer
	if environment == staging {
		issuer = "https://staging.cloud.compozy.com"
	}
	links, err := email.NewUnsubscribeLinks(issuer, key)
	if err != nil {
		return err
	}
	pooled, err := databaseURL(getenv, envNeonPooled, environment)
	if err != nil {
		return err
	}
	direct, err := databaseURL(getenv, envNeonDirect, environment)
	if err != nil {
		return err
	}
	db, err := store.Open(ctx, store.Config{PooledDSN: pooled, DirectDSN: direct})
	if err != nil {
		return err
	}
	defer db.Close()

	n, err := deliverBroadcast(ctx, db, resend, news, links)
	if err != nil {
		return fmt.Errorf("broadcast stopped after %d submitted messages: %w", n, err)
	}
	_, err = fmt.Fprintf(out, "Submitted %d messages to Resend.\n", n)
	return err
}

func deliverBroadcast(
	ctx context.Context, db *store.Store, resend *email.Resend, news email.Broadcast, links *email.UnsubscribeLinks,
) (int, error) {
	contacts, err := db.AdminOptedInContacts(ctx)
	if err != nil {
		return 0, err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	sent := 0
	for _, contact := range contacts {
		select {
		case <-ctx.Done():
			return sent, ctx.Err()
		case <-ticker.C:
		}

		err := db.Transaction(ctx, func(q *store.Queries) error {
			account, err := q.LockAuthAccount(ctx, contact.ID)
			if errors.Is(err, store.ErrAuthNotFound) {
				return nil
			}
			if err != nil || !account.MarketingOptIn || account.State == "deleting" {
				return err
			}
			if err := resend.SendBroadcast(ctx, news, account.ID, account.Email, links); err != nil {
				return err
			}
			sent++
			return nil
		})
		if err != nil {
			return sent, err
		}
	}
	return sent, nil
}

func readBroadcastText(path string) (_ string, retErr error) {
	const limit = 256 << 10
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { retErr = errors.Join(retErr, file.Close()) }()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return "", err
	}
	if len(content) > limit {
		return "", errors.New("broadcast text file exceeds 256 KiB")
	}
	return string(content), nil
}
