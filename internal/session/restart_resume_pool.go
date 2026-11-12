package session

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// restartResumeConcurrency bounds how many interrupted sessions resume at once after a restart.
// Each resume starts a provider process, so the pool trades restart time against host load.
const restartResumeConcurrency = 4

// restartResumeItem is one interrupted session and the earlier items that must settle first.
type restartResumeItem struct {
	id    string
	after []int
}

// planRestartResumes orders interrupted user sessions for the bounded resume pool. Parents come
// before their children and always-on instance mains before other sessions of the same depth.
// A child waits for its parent, and sessions sharing a worktree keep their serial order, because
// both pairs contend on the same owner state. Every dependency points to an earlier item.
func planRestartResumes(infos []*Info) []restartResumeItem {
	candidates := make([]*Info, 0, len(infos))
	byID := make(map[string]*Info, len(infos))
	for _, info := range infos {
		if info == nil || info.Type != SessionTypeUser || info.StopReason != store.StopShutdown {
			continue
		}
		candidates = append(candidates, info)
		byID[info.ID] = info
	}
	depths := make(map[string]int, len(candidates))
	for _, info := range candidates {
		depths[info.ID] = restartResumeDepth(info, byID)
	}
	slices.SortStableFunc(candidates, func(a, b *Info) int {
		if order := cmp.Compare(depths[a.ID], depths[b.ID]); order != 0 {
			return order
		}
		return cmp.Compare(restartResumeRank(a), restartResumeRank(b))
	})
	indexes := make(map[string]int, len(candidates))
	lastInWorktree := make(map[string]int)
	items := make([]restartResumeItem, 0, len(candidates))
	for index, info := range candidates {
		item := restartResumeItem{id: info.ID}
		if parent, ok := indexes[restartResumeParent(info)]; ok {
			item.after = append(item.after, parent)
		}
		if worktree := strings.TrimSpace(info.WorktreeID); worktree != "" {
			if previous, ok := lastInWorktree[worktree]; ok && !slices.Contains(item.after, previous) {
				item.after = append(item.after, previous)
			}
			lastInWorktree[worktree] = index
		}
		indexes[info.ID] = index
		items = append(items, item)
	}
	return items
}

func restartResumeParent(info *Info) string {
	if info == nil || info.Lineage == nil {
		return ""
	}
	return strings.TrimSpace(info.Lineage.ParentSessionID)
}

// restartResumeDepth counts interrupted ancestors; the bound keeps a corrupt lineage cycle finite.
func restartResumeDepth(info *Info, byID map[string]*Info) int {
	depth := 0
	for current := byID[restartResumeParent(info)]; current != nil && depth < len(byID); depth++ {
		current = byID[restartResumeParent(current)]
	}
	return depth
}

func restartResumeRank(info *Info) int {
	if strings.TrimSpace(info.InstanceID) != "" {
		return 0
	}
	return 1
}

// runRestartResumes runs resume for every item with at most limit in flight, starting an item only
// after the items it depends on have settled. A failed resume still settles its item: the session
// stays stopped for the operator and its dependents proceed, as in the former serial loop.
// Cancellation stops new starts and waits for the resumes already running.
func runRestartResumes(
	ctx context.Context,
	items []restartResumeItem,
	limit int,
	resume func(context.Context, string),
) error {
	limit = max(limit, 1)
	started := make([]bool, len(items))
	settled := make([]bool, len(items))
	completions := make(chan int, len(items))
	var workers sync.WaitGroup
	running, remaining := 0, len(items)
	for remaining > 0 {
		for index := 0; index < len(items) && running < limit && ctx.Err() == nil; index++ {
			if started[index] || !restartResumeReady(items[index], settled) {
				continue
			}
			started[index] = true
			running++
			workers.Go(func() {
				resume(ctx, items[index].id)
				completions <- index
			})
		}
		if running == 0 {
			break
		}
		index := <-completions
		settled[index] = true
		running--
		remaining--
	}
	workers.Wait()
	return ctx.Err()
}

func restartResumeReady(item restartResumeItem, settled []bool) bool {
	for _, dependency := range item.after {
		if !settled[dependency] {
			return false
		}
	}
	return true
}
