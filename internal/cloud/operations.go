// SPDX-License-Identifier: FSL-1.1-MIT

package cloud

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store"
)

func (a *App) RunOperationalMail(ctx context.Context) error {
	now := a.now()
	messages, err := a.Store.DueOperationalMail(ctx, now, "")
	if err != nil {
		return err
	}
	var failures error
	for _, candidate := range messages {
		if ctx.Err() != nil {
			return errors.Join(failures, ctx.Err())
		}
		err := a.Store.DeliverOperationalMail(ctx, now, candidate, func(message store.OperationalMail) error {
			key := fmt.Sprintf("cloud-ops/%x", sha256.Sum256([]byte(message.AccountID+"/"+message.Key)))
			return a.mailer.Send(ctx, key, operationalMail(message))
		})
		failures = errors.Join(failures, err)
	}
	return failures
}

func operationalMail(message store.OperationalMail) email.Message {
	mail := email.Message{To: message.Email}
	date := message.DeleteAt.UTC().Format("January 2, 2006")
	switch message.Kind {
	case "pro_ended":
		mail.Subject = "Pro ended"
		mail.Text = "Your Compozy Pro subscription ended. Your hosted routes will be deleted on " + date +
			" (UTC). Resubscribe before that date to keep them."
	case "pro_reminder":
		mail.Subject = "Your Compozy hosted routes will be deleted in 2 days"
		mail.Text = "Your hosted routes will be deleted on " + date + " (UTC). Resubscribe to keep them."
	case "offline":
		mail.Subject = "A Compozy computer has been offline for 10 days"
		mail.Text = fmt.Sprintf("%s has been offline for 10 days. Its hosted route will be deleted on %s (UTC). "+
			"Reconnect the computer to keep its hosted route.", message.ComputerName, date)
	}
	return mail
}

func (a *App) runOperations(ctx context.Context) {
	ticks := time.Tick(time.Minute)
	for {
		a.reportOperation(ctx, "transactional_mail", a.runMailBatch)
		a.reportOperation(ctx, "metrics_export", a.publishMetrics)
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}

func (a *App) runMailBatch(ctx context.Context) error {
	if a.mailer == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	return a.RunOperationalMail(ctx)
}

func (a *App) publishMetrics(ctx context.Context) error {
	if a.publisher == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	now := a.now()
	samples, err := a.operationalSamples(ctx, now)
	if err != nil {
		return err
	}
	return a.publisher.Publish(ctx, now, samples)
}

func (a *App) operationalSamples(ctx context.Context, now time.Time) ([]observe.Sample, error) {
	gauges, err := a.Store.OperationalGauges(ctx, now, a.monthly, a.annual)
	if err != nil {
		return nil, err
	}
	gauges = append(gauges, observe.Sample{Name: "cloud_control_links", Value: float64(a.Control.ComputersConnected())})
	return append(a.Metrics.Samples(), gauges...), nil
}

func (a *App) reportOperation(ctx context.Context, operation string, fn func(context.Context) error) {
	if err := fn(ctx); err != nil && ctx.Err() == nil {
		a.logger.WarnContext(ctx, "Cloud operation failed", "operation", operation)
		a.reporter.Capture(ctx, operation, observe.Client{})
	}
}
