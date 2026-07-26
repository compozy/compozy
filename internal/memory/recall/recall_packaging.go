package recall

import (
	"crypto/sha256"
	"encoding/hex"

	"fmt"

	"sort"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func applyShadowRules(ranked []rankedCandidate) ([]rankedCandidate, []Shadow) {
	winners := make(map[string]rankedCandidate, len(ranked))
	shadows := make([]Shadow, 0)
	for _, candidate := range ranked {
		key := shadowKey(candidate.Candidate)
		if key == "" {
			winners[candidate.ChunkID] = candidate
			continue
		}
		current, exists := winners[key]
		if !exists {
			winners[key] = candidate
			continue
		}
		winner, loser := pickShadowWinner(current, candidate)
		winners[key] = winner
		shadows = append(shadows, Shadow{
			WinnerChunkID: winner.ChunkID,
			LoserChunkID:  loser.ChunkID,
			WorkspaceID:   winner.WorkspaceID,
			Scope:         winner.Scope,
			AgentName:     winner.AgentName,
			AgentTier:     winner.AgentTier,
			Type:          winner.Type,
			Slug:          winner.Slug,
		})
	}

	out := make([]rankedCandidate, 0, len(winners))
	for _, candidate := range winners {
		out = append(out, candidate)
	}
	sortRanked(out)
	return out, shadows
}

func shadowKey(candidate Candidate) string {
	typeName := strings.TrimSpace(string(candidate.Type.Normalize()))
	slug := strings.TrimSpace(candidate.Slug)
	if typeName == "" || slug == "" {
		return ""
	}
	return typeName + "::" + slug
}

func pickShadowWinner(left rankedCandidate, right rankedCandidate) (rankedCandidate, rankedCandidate) {
	if left.scopeDepth() != right.scopeDepth() {
		if left.scopeDepth() > right.scopeDepth() {
			return left, right
		}
		return right, left
	}
	if left.score >= right.score {
		return left, right
	}
	return right, left
}

func (candidate rankedCandidate) scopeDepth() int {
	switch candidate.Scope.Normalize() {
	case memcontract.ScopeAgent:
		if candidate.AgentTier.Normalize() == memcontract.AgentTierWorkspace {
			return 3
		}
		return 2
	case memcontract.ScopeWorkspace:
		return 1
	case memcontract.ScopeGlobal:
		return 0
	default:
		return 0
	}
}

func packageCandidates(ranked []rankedCandidate, topK int, now time.Time) memcontract.Packaged {
	if len(ranked) == 0 {
		return emptyPackage()
	}
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}

	blocks := groupBlocks(ranked, now)
	header := stableHeader(blocks, ranked)
	return memcontract.Packaged{Blocks: blocks, Header: header}
}

func groupBlocks(ranked []rankedCandidate, now time.Time) []memcontract.Block {
	groups := make(map[string][]memcontract.PackagedEntry)
	order := make([]string, 0)
	blockMeta := make(map[string]struct {
		scope memcontract.Scope
		tier  memcontract.AgentTier
		depth int
	})
	for _, candidate := range ranked {
		key := blockKey(candidate.Candidate)
		if _, exists := groups[key]; !exists {
			order = append(order, key)
			blockMeta[key] = struct {
				scope memcontract.Scope
				tier  memcontract.AgentTier
				depth int
			}{scope: candidate.Scope.Normalize(), tier: candidate.AgentTier.Normalize(), depth: candidate.scopeDepth()}
		}
		groups[key] = append(groups[key], packagedEntry(candidate, now))
	}
	sort.SliceStable(order, func(i, j int) bool {
		left := blockMeta[order[i]]
		right := blockMeta[order[j]]
		if left.depth != right.depth {
			return left.depth < right.depth
		}
		return order[i] < order[j]
	})

	blocks := make([]memcontract.Block, 0, len(order))
	for _, key := range order {
		entries := groups[key]
		sort.SliceStable(entries, func(i, j int) bool {
			return entries[i].ID < entries[j].ID
		})
		meta := blockMeta[key]
		blocks = append(blocks, memcontract.Block{
			Scope:     meta.scope,
			AgentTier: meta.tier,
			Entries:   entries,
		})
	}
	return blocks
}

func blockKey(candidate Candidate) string {
	return string(candidate.Scope.Normalize()) + "::" + string(candidate.AgentTier.Normalize())
}

func packagedEntry(candidate rankedCandidate, now time.Time) memcontract.PackagedEntry {
	age := ageDays(candidate.ModTime, now)
	return memcontract.PackagedEntry{
		ID:              candidate.ChunkID,
		Filename:        candidate.Filename,
		Title:           candidate.Title,
		Type:            candidate.Type.Normalize(),
		WorkspaceID:     candidate.WorkspaceID,
		Body:            candidate.Body,
		ModTime:         candidate.ModTime.UTC(),
		AgeDays:         age,
		StalenessBanner: stalenessBanner(age),
		WhyRecalled:     append([]string(nil), candidate.why...),
	}
}

func stableHeader(blocks []memcontract.Block, ranked []rankedCandidate) memcontract.CacheStableHeader {
	parts := make([]string, 0, len(ranked))
	for _, candidate := range ranked {
		parts = append(parts, strings.Join([]string{
			candidate.ChunkID,
			candidate.ContentHash,
			string(candidate.Scope.Normalize()),
			string(candidate.AgentTier.Normalize()),
		}, "|"))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	hash := hex.EncodeToString(sum[:])
	return memcontract.CacheStableHeader{
		Text:        fmt.Sprintf("AGH memory recall v1 entries=%d hash=%s", packagedBlockEntryCount(blocks), hash),
		ContentHash: hash,
	}
}

func emptyPackage() memcontract.Packaged {
	return memcontract.Packaged{
		Blocks: []memcontract.Block{},
		Header: memcontract.CacheStableHeader{Text: "AGH memory recall v1 entries=0 hash=", ContentHash: ""},
	}
}
