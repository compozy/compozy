package acp

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

// PromptResourceLink is one verified file or folder mention delivered as an
// ACP resource_link block. No content is read or embedded.
type PromptResourceLink struct {
	Name string // path relative to the session working directory; folders end in "/"
	URI  string // file:// + percent-encoded absolute path; folders end in "/"
}

// PromptMentionDelivery carries the session-layer mention counts recorded in the
// delivery manifest. Link counts are derived from PromptRequest.MentionLinks.
type PromptMentionDelivery struct {
	Sessions          int      // referenced sessions rendered in the references block
	SessionIDs        []string // ids of the rendered sessions, in block order
	ReferencesForm    string   // reference | inline | unavailable; empty without sessions
	DroppedOverCap    int      // tokens left as text because a per-prompt cap was reached
	DroppedAtDispatch int      // admitted mentions dropped by the dispatch re-check
}

// Delivered mention block kinds.
const (
	DeliveredMentionBlockSessionReferences = "session_references"
	DeliveredMentionBlockResourceLink      = "resource_link"
)

// DeliveredMentions records the mention blocks sent with one prompt.
type DeliveredMentions struct {
	Links             int                     `json:"links"`
	Sessions          int                     `json:"sessions"`
	ReferencesForm    string                  `json:"references_form,omitempty"`
	DroppedOverCap    int                     `json:"dropped_over_cap"`
	DroppedAtDispatch int                     `json:"dropped_at_dispatch"`
	Blocks            []DeliveredMentionBlock `json:"blocks,omitempty"`
}

// DeliveredMentionBlock is one delivered mention block, in prompt order. It never
// carries an absolute path: links record only their working-directory-relative
// name, and the references block records its form, session ids, and size.
type DeliveredMentionBlock struct {
	Kind       string   `json:"kind"`
	Name       string   `json:"name,omitempty"`
	Form       string   `json:"form,omitempty"`
	SessionIDs []string `json:"session_ids,omitempty"`
	Bytes      int64    `json:"bytes,omitzero"`
}

// NewPromptResourceLink builds the resource link for a verified path mention.
// relPath is relative to the working directory and absPath is its absolute
// location; folder links end both fields in "/".
func NewPromptResourceLink(relPath, absPath string, folder bool) PromptResourceLink {
	name := filepath.ToSlash(relPath)
	location := filepath.ToSlash(absPath)
	if !strings.HasPrefix(location, "/") {
		// Windows drive paths (C:/x) still need the empty authority form file:///C:/x.
		location = "/" + location
	}
	if folder {
		name = withTrailingSlash(name)
		location = withTrailingSlash(location)
	}
	return PromptResourceLink{Name: name, URI: fileURI(location)}
}

func fileURI(slashPath string) string {
	return (&url.URL{Scheme: "file", Path: slashPath}).String()
}

func withTrailingSlash(value string) string {
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}

// mentionContentBlocks returns the session-references text block (when present)
// followed by one resource_link block per mention link, in delivery order.
func mentionContentBlocks(req PromptRequest) []acpsdk.ContentBlock {
	blocks := make([]acpsdk.ContentBlock, 0, 1+len(req.MentionLinks))
	if references := strings.TrimSpace(req.SessionReferences); references != "" {
		blocks = append(blocks, acpsdk.TextBlock(req.SessionReferences))
	}
	for _, link := range req.MentionLinks {
		blocks = append(blocks, acpsdk.ResourceLinkBlock(link.Name, link.URI))
	}
	return blocks
}

// deliveredMentions returns the manifest mention counts and blocks, or nil when
// the prompt carried no mention activity.
func deliveredMentions(req PromptRequest) *DeliveredMentions {
	delivery := req.MentionDelivery
	form := strings.TrimSpace(delivery.ReferencesForm)
	result := DeliveredMentions{
		Links:             len(req.MentionLinks),
		Sessions:          delivery.Sessions,
		ReferencesForm:    form,
		DroppedOverCap:    delivery.DroppedOverCap,
		DroppedAtDispatch: delivery.DroppedAtDispatch,
	}
	if references := strings.TrimSpace(req.SessionReferences); references != "" {
		result.Blocks = append(result.Blocks, DeliveredMentionBlock{
			Kind:       DeliveredMentionBlockSessionReferences,
			Form:       form,
			SessionIDs: slices.Clone(delivery.SessionIDs),
			Bytes:      int64(len(req.SessionReferences)),
		})
	}
	for _, link := range req.MentionLinks {
		result.Blocks = append(result.Blocks, DeliveredMentionBlock{
			Kind: DeliveredMentionBlockResourceLink,
			Name: link.Name,
		})
	}
	if len(result.Blocks) == 0 && result.Sessions == 0 && result.ReferencesForm == "" &&
		result.DroppedOverCap == 0 && result.DroppedAtDispatch == 0 {
		return nil
	}
	return &result
}

func cloneDeliveredMentions(mentions *DeliveredMentions) *DeliveredMentions {
	if mentions == nil {
		return nil
	}
	cloned := *mentions
	cloned.Blocks = slices.Clone(mentions.Blocks)
	for index := range cloned.Blocks {
		cloned.Blocks[index].SessionIDs = slices.Clone(cloned.Blocks[index].SessionIDs)
	}
	return &cloned
}
