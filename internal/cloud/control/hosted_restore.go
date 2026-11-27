// SPDX-License-Identifier: FSL-1.1-MIT

package control

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/cloud/cloudflare"
)

func (s *Service) SweepAfterRestore(ctx context.Context) error {
	if s.config.HostedTunnels == nil {
		return errors.New("cloud restore requires Cloudflare hosted routes")
	}
	if err := s.SweepHostedTunnels(ctx); err != nil {
		return err
	}
	rows, err := s.db.RestoreKnownTunnels(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]cloudflare.ManagedTunnel, len(rows))
	for _, row := range rows {
		known[row.Hostname] = cloudflare.ManagedTunnel{
			Hostname:   row.Hostname,
			Generation: row.Generation,
			ID:         row.CloudflareID,
		}
	}
	remote, err := s.config.HostedTunnels.ManagedTunnels(ctx)
	if err != nil {
		return err
	}
	for _, item := range remote {
		saved := known[item.Hostname]
		if saved.Generation == item.Generation && (saved.ID == "" || saved.ID == item.ID) {
			continue
		}
		if err := s.config.HostedTunnels.DeleteRestoreOrphan(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
