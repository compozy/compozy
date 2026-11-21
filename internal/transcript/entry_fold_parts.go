package transcript

import (
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// ProjectedPartChange is an ordered part update and any new text for that part.
type ProjectedPartChange struct {
	Index  int
	Before UIMessagePart
	Part   UIMessagePart
	Text   string
}

type partChanges struct {
	previousCount int
	before        map[int]UIMessagePart
	text          map[int][]string
}

// NewEntryPartFold reduces assistant events without accumulating the streamed text in memory.
func NewEntryPartFold(identity EntryIdentity) (*EntryFold, error) {
	fold, err := NewEntryFold(identity)
	if err != nil {
		return nil, err
	}
	fold.builder.projectParts = true
	return fold, nil
}

// AppendPartChanges returns only part changes made by this batch on an entry part fold.
func (f *EntryFold) AppendPartChanges(events []store.SessionEvent) ([]ProjectedPartChange, error) {
	b := f.builder
	changes := &partChanges{
		previousCount: len(b.parts),
		before:        make(map[int]UIMessagePart),
		text:          make(map[int][]string),
	}
	b.partChanges = changes
	err := f.Append(events)
	b.partChanges = nil
	if err != nil {
		return nil, err
	}
	result := make([]ProjectedPartChange, 0, len(changes.before)+len(b.parts)-changes.previousCount)
	for _, index := range slices.Sorted(maps.Keys(changes.before)) {
		result = append(result, ProjectedPartChange{
			Index:  index,
			Before: changes.before[index],
			Part:   b.parts[index],
			Text:   strings.Join(changes.text[index], ""),
		})
	}
	for index := changes.previousCount; index < len(b.parts); index++ {
		result = append(result, ProjectedPartChange{
			Index: index, Part: b.parts[index], Text: strings.Join(changes.text[index], ""),
		})
	}
	return result, nil
}

// PartHeader returns the message envelope and the number of projected parts.
func (f *EntryFold) PartHeader() (UIMessage, int) {
	return f.builder.messageHeader(), len(f.builder.parts)
}

func (b *uiMessageBuilder) partForUpdate(index int) *UIMessagePart {
	if changes := b.partChanges; changes != nil && index < changes.previousCount {
		if _, ok := changes.before[index]; !ok {
			changes.before[index] = b.parts[index]
		}
	}
	return &b.parts[index]
}
