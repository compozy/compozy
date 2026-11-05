package github

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"
)

const pullBatchWindow = 5 * time.Millisecond
const maxPullBatchBranches = 20

type pullBatchKey struct {
	repository string
	credential [32]byte
}

type pullBatch struct {
	branches []string
	done     chan struct{}
	pages    map[string][2]graphPage
	err      error
}

func (p *Provider) batchedPullPages(ctx context.Context, repo repository, branch, token string) ([2]graphPage, error) {
	key := pullBatchKey{strings.ToLower(repo.owner + "/" + repo.name), sha256.Sum256([]byte(token))}
	if err := ctx.Err(); err != nil {
		return [2]graphPage{}, err
	}
	p.batchMu.Lock()
	if p.batchClosed {
		p.batchMu.Unlock()
		return [2]graphPage{}, context.Canceled
	}
	if p.batchCtx == nil {
		p.batchCtx, p.batchCancel = context.WithCancel(context.Background())
	}
	if p.batches == nil {
		p.batches = make(map[pullBatchKey]*pullBatch)
	}
	batch := p.batches[key]
	leader := batch == nil || len(batch.branches) >= maxPullBatchBranches
	if leader {
		batch = &pullBatch{done: make(chan struct{})}
		p.batches[key] = batch
	}
	batch.branches = append(batch.branches, branch)
	if leader {
		p.batchWG.Go(func() { p.runPullBatch(ctx, key, repo, token, batch) })
	}
	p.batchMu.Unlock()
	select {
	case <-ctx.Done():
		return [2]graphPage{}, ctx.Err()
	case <-batch.done:
		return batch.pages[branch], batch.err
	}
}

func (p *Provider) runPullBatch(
	ctx context.Context,
	key pullBatchKey,
	repo repository,
	token string,
	batch *pullBatch,
) {
	// A test flush replaces the window so joining callers never race the timer.
	var window <-chan time.Time
	if p.batchFlush == nil {
		timer := time.NewTimer(pullBatchWindow)
		defer timer.Stop()
		window = timer.C
	}
	select {
	case <-window:
	case <-p.batchFlush:
	case <-p.batchCtx.Done():
		batch.err = p.batchCtx.Err()
		close(batch.done)
		return
	}
	p.batchMu.Lock()
	if p.batches[key] == batch {
		delete(p.batches, key)
	}
	branches := append([]string(nil), batch.branches...)
	p.batchMu.Unlock()
	// Each caller keeps its cancellation; one disconnect must not cancel its siblings' lookup.
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
	defer cancel()
	stop := context.AfterFunc(p.batchCtx, cancel)
	defer stop()
	query, variables := pullBatchQuery(repo, branches)
	result, err := p.graphRequest(lookupCtx, token, query, variables)
	batch.pages = make(map[string][2]graphPage, len(branches))
	batch.err = err
	if err == nil {
		for i, branch := range branches {
			open, terminal := pullBatchAliases(i, len(branches))
			pages := result.Data.Repository.Pages
			openPage, ok := pages[open]
			if !ok {
				batch.err = errors.New("github forge: missing open pull request batch result")
				break
			}
			terminalPage, ok := pages[terminal]
			if !ok {
				batch.err = errors.New("github forge: missing terminal pull request batch result")
				break
			}
			batch.pages[branch] = [2]graphPage{openPage, terminalPage}
		}
	}
	close(batch.done)
}

func pullBatchAliases(index, count int) (string, string) {
	if count == 1 {
		return "pullRequests", "terminalPullRequests"
	}
	return fmt.Sprintf("branch%dOpen", index), fmt.Sprintf("branch%dTerminal", index)
}

func pullBatchQuery(repo repository, branches []string) (string, map[string]any) {
	var declarations, fields strings.Builder
	variables := graphPageVariables(repo, nil)
	delete(variables, "after")
	for i, branch := range branches {
		variable := fmt.Sprintf("branch%d", i)
		variables[variable] = branch
		fmt.Fprintf(&declarations, ",$%s:String!", variable)
		open, terminal := pullBatchAliases(i, len(branches))
		for group, alias := range []string{open, terminal} {
			states := graphStateOpen
			if group == 1 {
				states = "MERGED,CLOSED"
			}
			fmt.Fprintf(
				&fields,
				`%s:pullRequests(headRefName:$%s,states:[%s],first:50,orderBy:{field:UPDATED_AT,direction:DESC}){
 nodes{number title url state isDraft merged headRefOid baseRefName updatedAt
 reviewDecision mergeable headRepository{nameWithOwner}}
 pageInfo{hasNextPage endCursor}}`,
				alias,
				variable,
				states,
			)
		}
	}
	return "query($owner:String!,$name:String!" + declarations.String() +
		"){repository(owner:$owner,name:$name){" + fields.String() + "}}", variables
}

func (p *Provider) closeBatches() {
	p.batchMu.Lock()
	p.batchClosed = true
	if p.batchCancel != nil {
		p.batchCancel()
	}
	p.batchMu.Unlock()
	p.batchWG.Wait()
}
