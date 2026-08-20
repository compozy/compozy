package cmdpalette

import (
	"encoding/json"
	"maps"
)

func cloneViewPayload(payload ViewPayload) ViewPayload {
	cloned := payload
	cloned.Chrome = cloneViewChrome(payload.Chrome)
	cloned.Sections = cloneViewSections(payload.Sections)
	cloned.Chips = cloneViewChips(payload.Chips)
	cloned.Empty = cloneEmptyState(payload.Empty)
	cloned.Detail = cloneDetailBody(payload.Detail)
	cloned.Form = cloneFormBody(payload.Form)
	cloned.Grid = cloneGridBody(payload.Grid)
	return cloned
}

func cloneViewChrome(chrome *ViewChrome) *ViewChrome {
	if chrome == nil {
		return nil
	}
	cloned := *chrome
	cloned.SearchText = cloneString(chrome.SearchText)
	cloned.Filtering = cloneBool(chrome.Filtering)
	if chrome.Pagination != nil {
		pagination := *chrome.Pagination
		cloned.Pagination = &pagination
	}
	return &cloned
}

func cloneViewSections(sections []Section) []Section {
	if sections == nil {
		return nil
	}
	cloned := make([]Section, len(sections))
	for index, section := range sections {
		cloned[index] = Section{Title: section.Title, Rows: cloneViewRows(section.Rows)}
	}
	return cloned
}

func cloneViewRows(rows []Row) []Row {
	if rows == nil {
		return nil
	}
	cloned := make([]Row, len(rows))
	for index, row := range rows {
		cloned[index] = row
		cloned[index].Keywords = append([]string(nil), row.Keywords...)
		cloned[index].Accessories = append([]string(nil), row.Accessories...)
		cloned[index].Badge = cloneViewBadge(row.Badge)
		cloned[index].Detail = cloneDetailBody(row.Detail)
		cloned[index].Actions = cloneRowActions(row.Actions)
		cloned[index].Requires = maps.Clone(row.Requires)
	}
	return cloned
}

func cloneViewChips(chips []Chip) []Chip {
	if chips == nil {
		return nil
	}
	cloned := make([]Chip, len(chips))
	for index, chip := range chips {
		cloned[index] = chip
		cloned[index].Count = cloneInt(chip.Count)
		cloned[index].Requires = maps.Clone(chip.Requires)
	}
	return cloned
}

func cloneEmptyState(empty *EmptyState) *EmptyState {
	if empty == nil {
		return nil
	}
	cloned := *empty
	return &cloned
}

func cloneDetailBody(detail *DetailBody) *DetailBody {
	if detail == nil {
		return nil
	}
	cloned := *detail
	cloned.Metadata = append([]MetaField(nil), detail.Metadata...)
	for index, field := range cloned.Metadata {
		cloned.Metadata[index].Requires = maps.Clone(field.Requires)
	}
	cloned.Actions = cloneRowActions(detail.Actions)
	return &cloned
}

func cloneFormBody(form *FormBody) *FormBody {
	if form == nil {
		return nil
	}
	cloned := *form
	cloned.Fields = append([]FormField(nil), form.Fields...)
	for index, field := range cloned.Fields {
		cloned.Fields[index].Options = append([]string(nil), field.Options...)
		cloned.Fields[index].Requires = maps.Clone(field.Requires)
	}
	if form.Submit != nil {
		submit := cloneRowAction(*form.Submit)
		cloned.Submit = &submit
	}
	return &cloned
}

func cloneGridBody(grid *GridBody) *GridBody {
	if grid == nil {
		return nil
	}
	cloned := *grid
	cloned.Sections = append([]GridSection(nil), grid.Sections...)
	for sectionIndex, section := range cloned.Sections {
		cloned.Sections[sectionIndex].Tiles = append([]GridTile(nil), section.Tiles...)
		for tileIndex, tile := range cloned.Sections[sectionIndex].Tiles {
			cloned.Sections[sectionIndex].Tiles[tileIndex].Badge = cloneViewBadge(tile.Badge)
			cloned.Sections[sectionIndex].Tiles[tileIndex].Actions = cloneRowActions(tile.Actions)
			cloned.Sections[sectionIndex].Tiles[tileIndex].Requires = maps.Clone(tile.Requires)
		}
	}
	return &cloned
}

func cloneRowActions(actions []RowAction) []RowAction {
	if actions == nil {
		return nil
	}
	cloned := make([]RowAction, len(actions))
	for index, action := range actions {
		cloned[index] = cloneRowAction(action)
	}
	return cloned
}

func cloneRowAction(action RowAction) RowAction {
	if action.Confirmation != nil {
		confirmation := *action.Confirmation
		action.Confirmation = &confirmation
	}
	if action.Action != nil {
		copied := cloneDescriptor(Descriptor{Action: *action.Action}).Action
		action.Action = &copied
	}
	action.Requires = maps.Clone(action.Requires)
	return action
}

func cloneViewBadge(badge *ViewBadge) *ViewBadge {
	if badge == nil {
		return nil
	}
	cloned := *badge
	return &cloned
}

func cloneViewEffects(effects []Effect) []Effect {
	if effects == nil {
		return nil
	}
	cloned := make([]Effect, len(effects))
	for index, effect := range effects {
		cloned[index] = cloneViewEffect(effect)
	}
	return cloned
}

func cloneViewEffect(effect Effect) Effect {
	if effect.Toast != nil {
		toast := *effect.Toast
		effect.Toast = &toast
	}
	if effect.Copy != nil {
		copyEffect := *effect.Copy
		effect.Copy = &copyEffect
	}
	if effect.OpenURL != nil {
		openURL := *effect.OpenURL
		effect.OpenURL = &openURL
	}
	if effect.OpenApp != nil {
		openApp := *effect.OpenApp
		effect.OpenApp = &openApp
	}
	if effect.PickFiles != nil {
		pickFiles := *effect.PickFiles
		effect.PickFiles = &pickFiles
	}
	return effect
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func clonePatchOps(ops []PatchOp) []PatchOp {
	if ops == nil {
		return nil
	}
	cloned := make([]PatchOp, len(ops))
	for index, operation := range ops {
		cloned[index] = operation
		cloned[index].Value = append(json.RawMessage(nil), operation.Value...)
	}
	return cloned
}
