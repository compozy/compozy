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
					if err := s.populateMCPServerStatuses(
						probeCtx,
						&items[index].item,
						items[index].entry,
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

func mcpCollectionProbeError(ctx context.Context, errs <-chan error) error {
	select {
	case err := <-errs:
		return err
	default:
		return ctx.Err()
	}
}
