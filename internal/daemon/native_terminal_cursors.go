package daemon

import (
	"container/list"
	"strings"
	"sync"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const maxTerminalReadCursors = 1024

type terminalReadCursor struct {
	key string
	seq uint64
}

type terminalReadCursors struct {
	mu      sync.Mutex
	values  map[string]*list.Element
	ordered list.List
}

func terminalReadCursorKey(scope toolspkg.Scope, terminalID string) string {
	return strings.Join([]string{scope.ProfileID, scope.SessionID, terminalID}, "\x00")
}

func (c *terminalReadCursors) Load(key string) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	element := c.values[key]
	if element == nil {
		return 0
	}
	c.ordered.MoveToBack(element)
	return terminalReadCursorValue(element.Value).seq
}

func (c *terminalReadCursors) Store(key string, seq uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.values == nil {
		c.values = make(map[string]*list.Element)
	}
	if element := c.values[key]; element != nil {
		element.Value = terminalReadCursor{key: key, seq: seq}
		c.ordered.MoveToBack(element)
		return
	}
	c.values[key] = c.ordered.PushBack(terminalReadCursor{key: key, seq: seq})
	if c.ordered.Len() <= maxTerminalReadCursors {
		return
	}
	oldest := c.ordered.Front()
	delete(c.values, terminalReadCursorValue(oldest.Value).key)
	c.ordered.Remove(oldest)
}

func terminalReadCursorValue(value any) terminalReadCursor {
	cursor, ok := value.(terminalReadCursor)
	if !ok {
		panic("terminal read cursor: order contains an invalid value")
	}
	return cursor
}

func (c *terminalReadCursors) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if element := c.values[key]; element != nil {
		delete(c.values, key)
		c.ordered.Remove(element)
	}
}
