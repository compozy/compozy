package heartbeat

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
)

func ProposalFingerprint(proposal TriageProposal) string {
	keys := slices.Clone(proposal.ItemKeys)
	slices.Sort(keys)
	keys = slices.Compact(keys)
	data := []byte("keys:")
	if len(keys) == 0 {
		data = append([]byte("title:"), strings.ToLower(strings.Join(strings.Fields(proposal.Title), " "))...)
	} else {
		for _, key := range keys {
			data = strconv.AppendQuote(data, key)
		}
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
