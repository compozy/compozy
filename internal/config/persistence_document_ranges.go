package config

import (
	"bytes"
	"errors"

	"math"

	"strings"

	tomlast "github.com/pelletier/go-toml/v2/unstable"
)

func tableHeaderRange(source []byte, node *tomlast.Node) tomlast.Range {
	if len(source) == 0 {
		return tomlast.Range{}
	}

	start := rangeStart(node.Raw)
	keyIter := node.Key()
	if keyIter.Next() {
		start = rangeStart(keyIter.Node().Raw)
	}
	if start < 0 {
		start = 0
	}
	if start > len(source) {
		start = len(source)
	}

	lineStart := 0
	if start > 0 {
		lineStart = bytes.LastIndexByte(source[:start], '\n') + 1
	}
	if idx := bytes.IndexByte(source[lineStart:], '['); idx >= 0 {
		start = lineStart + idx
	} else {
		start = lineStart
	}

	end := start
	for end < len(source) && source[end] != '\n' {
		end++
	}
	if end < len(source) {
		end++
	}

	return tomlast.Range{Offset: uint32(start), Length: uint32(end - start)}
}

func nodePath(node *tomlast.Node) ([]string, error) {
	keys := make([]string, 0)
	it := node.Key()
	for it.Next() {
		key := strings.TrimSpace(string(it.Node().Data))
		if key == "" {
			return nil, errors.New("config: TOML key path contains an empty segment")
		}
		keys = append(keys, key)
	}
	return keys, nil
}

func expressionRange(node *tomlast.Node) tomlast.Range {
	minStart := -1
	maxEnd := 0
	var walk func(*tomlast.Node)
	walk = func(current *tomlast.Node) {
		if !current.Valid() {
			return
		}
		if current.Raw.Length > 0 {
			start := rangeStart(current.Raw)
			end := rangeEnd(current.Raw)
			if minStart == -1 || start < minStart {
				minStart = start
			}
			if end > maxEnd {
				maxEnd = end
			}
		}
		children := current.Children()
		for children.Next() {
			walk(children.Node())
		}
	}
	walk(node)
	if minStart == -1 {
		return node.Raw
	}
	length := maxEnd - minStart
	if minStart < 0 || length < 0 || uint64(minStart) > math.MaxUint32 || uint64(length) > math.MaxUint32 {
		return node.Raw
	}
	return tomlast.Range{Offset: uint32(minStart), Length: uint32(length)}
}

func (d *overlayDocument) findKeyValue(path []string) *overlayExpression {
	for idx := range d.expressions {
		expr := &d.expressions[idx]
		if expr.kind == tomlast.KeyValue && pathsEqual(expr.path, path) {
			return expr
		}
	}
	return nil
}

func (d *overlayDocument) findTable(path []string) *overlayExpression {
	for idx := range d.expressions {
		expr := &d.expressions[idx]
		if expr.kind == tomlast.Table && pathsEqual(expr.path, path) {
			return expr
		}
	}
	return nil
}

func (d *overlayDocument) tableInsertOffset(path []string) (int, error) {
	for idx := range d.expressions {
		expr := d.expressions[idx]
		if expr.kind != tomlast.Table || !pathsEqual(expr.path, path) {
			continue
		}

		offset := rangeEnd(expr.raw)
		for nextIdx := idx + 1; nextIdx < len(d.expressions); nextIdx++ {
			next := d.expressions[nextIdx]
			switch next.kind {
			case tomlast.KeyValue, tomlast.Comment:
				if pathsEqual(next.containerPath, path) {
					offset = lineEndOffset(d.source, next.raw)
					continue
				}
				return offset, nil
			case tomlast.Table, tomlast.ArrayTable:
				return offset, nil
			}
		}
		return offset, nil
	}

	return 0, unsupportedTOMLMutation(path, "table does not exist")
}

func (d *overlayDocument) tableBlockIncludingNested(path []string) (overlayBlock, bool) {
	for idx := range d.expressions {
		expr := d.expressions[idx]
		if expr.kind != tomlast.Table || !pathsEqual(expr.path, path) {
			continue
		}

		block := overlayBlock{
			path:     clonePath(path),
			startIdx: idx,
			endIdx:   idx,
			start:    rangeStart(expr.raw),
			end:      rangeEnd(expr.raw),
		}

		for nextIdx := idx + 1; nextIdx < len(d.expressions); nextIdx++ {
			next := d.expressions[nextIdx]
			switch next.kind {
			case tomlast.KeyValue, tomlast.Comment:
				if pathsEqual(next.containerPath, path) || pathHasPrefix(next.containerPath, path) {
					block.endIdx = nextIdx
					block.end = lineEndOffset(d.source, next.raw)
					continue
				}
				return block, true
			case tomlast.Table, tomlast.ArrayTable:
				if pathHasPrefix(next.path, path) && len(next.path) > len(path) {
					block.endIdx = nextIdx
					block.end = lineEndOffset(d.source, next.raw)
					continue
				}
				return block, true
			}
		}
		return block, true
	}
	return overlayBlock{}, false
}

// descendantTableBlocks treats an implicit parent table as its explicit descendants.
func (d *overlayDocument) descendantTableBlocks(path []string) []overlayBlock {
	blocks := make([]overlayBlock, 0)
scan:
	for idx := 0; idx < len(d.expressions); idx++ {
		expr := d.expressions[idx]
		if expr.kind != tomlast.Table && expr.kind != tomlast.ArrayTable {
			continue
		}
		if !pathHasPrefix(expr.path, path) || len(expr.path) <= len(path) {
			continue
		}
		block := overlayBlock{
			path:     clonePath(expr.path),
			startIdx: idx,
			endIdx:   idx,
			start:    rangeStart(expr.raw),
			end:      rangeEnd(expr.raw),
		}
		for nextIdx := idx + 1; nextIdx < len(d.expressions); nextIdx++ {
			next := d.expressions[nextIdx]
			switch next.kind {
			case tomlast.KeyValue, tomlast.Comment:
				if pathHasPrefix(next.containerPath, path) {
					block.endIdx = nextIdx
					block.end = lineEndOffset(d.source, next.raw)
					continue
				}
				blocks = append(blocks, block)
				idx = block.endIdx
				continue scan
			case tomlast.Table, tomlast.ArrayTable:
				if pathHasPrefix(next.path, path) && len(next.path) > len(path) {
					block.endIdx = nextIdx
					block.end = lineEndOffset(d.source, next.raw)
					continue
				}
				blocks = append(blocks, block)
				idx = block.endIdx
				continue scan
			}
		}
		blocks = append(blocks, block)
		idx = block.endIdx
	}
	return blocks
}

func (d *overlayDocument) arrayTableBlocks(path []string) []overlayBlock {
	blocks := make([]overlayBlock, 0)
	for idx := 0; idx < len(d.expressions); idx++ {
		expr := d.expressions[idx]
		if expr.kind != tomlast.ArrayTable || !pathsEqual(expr.path, path) {
			continue
		}

		block := overlayBlock{
			path:     clonePath(path),
			startIdx: idx,
			endIdx:   idx,
			start:    rangeStart(expr.raw),
			end:      rangeEnd(expr.raw),
		}

		for nextIdx := idx + 1; nextIdx < len(d.expressions); nextIdx++ {
			next := d.expressions[nextIdx]
			if next.kind == tomlast.ArrayTable && pathsEqual(next.path, path) {
				break
			}
			if next.kind == tomlast.Table || next.kind == tomlast.ArrayTable {
				if pathHasPrefix(next.path, path) && len(next.path) > len(path) {
					block.endIdx = nextIdx
					block.end = rangeEnd(next.raw)
					continue
				}
				break
			}
			block.endIdx = nextIdx
			block.end = lineEndOffset(d.source, next.raw)
		}

		blocks = append(blocks, block)
		idx = block.endIdx
	}
	return blocks
}

func (d *overlayDocument) blockStringField(block overlayBlock, field string) (string, bool) {
	targetPath := append(clonePath(block.path), field)
	for idx := block.startIdx; idx <= block.endIdx && idx < len(d.expressions); idx++ {
		expr := d.expressions[idx]
		if expr.kind != tomlast.KeyValue || !pathsEqual(expr.path, targetPath) {
			continue
		}
		return decodeStringValue(d.source[rangeStart(expr.value):rangeEnd(expr.value)])
	}
	return "", false
}

func (d *overlayDocument) ensureNoScalarPrefix(path []string) error {
	for _, expr := range d.expressions {
		if expr.kind != tomlast.KeyValue {
			continue
		}
		if pathHasPrefix(path, expr.path) && len(expr.path) <= len(path) {
			return unsupportedTOMLMutation(path, "cannot descend through a scalar value")
		}
	}
	return nil
}
