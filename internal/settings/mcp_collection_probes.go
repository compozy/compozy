package settings

import (
	"context"
	"sync"
)

const maxConcurrentMCPRuntimeProbes = 4

type mcpCollectionItem struct {
	item  MCPServerItem
	entry mcpSourceEntry
}

func (s *service) populateMCPCollectionStatuses(
	ctx context.Context,
	items []mcpCollectionItem,
	skipRuntimeStatus, cachedRuntimeStatus bool, generation uint64,
) error {
	if len(items) == 0 {
		return nil
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan int)
	errs := make(chan error, 1)
	workerCount := min(maxConcurrentMCPRuntimeProbes, len(items))
	var workers sync.WaitGroup
	for range workerCount {
		workers.Go(func() {
			for {
				select {
				case <-probeCtx.Done():
					return
				case index, ok := <-jobs:
					if !ok {
						return
					}
					if err := s.populateMCPCollectionItem(
						probeCtx, &items[index], skipRuntimeStatus, cachedRuntimeStatus, generation,
					); err != nil {
						select {
						case errs <- err:
							cancel()
						case <-probeCtx.Done():
						}
						return
					}
				}
			}
		})
	}

	for index := range items {
		select {
		case jobs <- index:
		case <-probeCtx.Done():
			close(jobs)
			workers.Wait()
			return mcpCollectionProbeError(probeCtx, errs)
		}
	}
	close(jobs)
	workers.Wait()
	return mcpCollectionProbeError(probeCtx, errs)
}

// Pin live probes to the enablement generation used to collect their definitions.
func (s *service) mcpProbeSnapshot(passive bool) uint64 {
	if passive {
		return 0
	}
	s.mcpProbeMu.RLock()
	defer s.mcpProbeMu.RUnlock()
	return s.mcpProbeGeneration
}

func (s *service) populateMCPCollectionItem(
	ctx context.Context, item *mcpCollectionItem, skip, cached bool, generation uint64,
) error {
	if !skip && !cached {
		s.mcpProbeMu.RLock()
		defer s.mcpProbeMu.RUnlock()
		skip = s.mcpProbeGeneration != generation
	}
	return s.populateMCPServerStatuses(ctx, &item.item, item.entry, skip, cached)
}

func mcpCollectionProbeError(ctx context.Context, errs <-chan error) error {
	select {
	case err := <-errs:
		return err
	default:
		return ctx.Err()
	}
}
