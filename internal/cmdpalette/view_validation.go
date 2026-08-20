package cmdpalette

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	ViewContractVersion  = "v1"
	MaxViewComponents    = 1500
	MaxViewMountRows     = 150
	MaxViewPatchRows     = 500
	MaxViewWireBytes     = 256 * 1024
	MaxViewTextBytes     = 4 * 1024
	MaxDetailMarkdown    = 64 * 1024
	DetailTruncationMark = "\n\n[Content truncated]"
)

var validBadgeTones = map[string]struct{}{
	"neutral": {}, "info": {}, "success": {}, "warning": {}, "danger": {},
}

// CapabilityGapReporter records an authored element that the current host cannot render.
type CapabilityGapReporter interface {
	RecordCapabilityGap(path string, requires map[string]string)
}

// ValidateViewPayload validates and normalizes one payload before it crosses a transport.
func ValidateViewPayload(
	kind ViewKind,
	payload ViewPayload,
	capabilities map[string]string,
	reporter CapabilityGapReporter,
) (ViewPayload, error) {
	if payload.View != ViewContractVersion {
		return ViewPayload{}, viewValidationError("view", "must be %q", ViewContractVersion)
	}
	payload = cloneViewPayload(payload)
	if err := validateViewKindBody(kind, payload); err != nil {
		return ViewPayload{}, err
	}
	normalized, err := resolveViewCapabilities(payload, capabilities, reporter)
	if err != nil {
		return ViewPayload{}, err
	}
	truncateViewDetails(&normalized)
	if err := validateViewFields(normalized); err != nil {
		return ViewPayload{}, err
	}
	if components := countViewComponents(normalized); components > MaxViewComponents {
		return ViewPayload{}, viewValidationError(
			"payload", "has %d components; maximum is %d", components, MaxViewComponents,
		)
	}
	applyViewMountCap(&normalized)
	wire, err := json.Marshal(normalized)
	if err != nil {
		return ViewPayload{}, fmt.Errorf("cmd palette: encode validated view payload: %w", err)
	}
	if len(wire) > MaxViewWireBytes {
		return ViewPayload{}, viewValidationError(
			"payload", "is %d bytes; maximum is %d", len(wire), MaxViewWireBytes,
		)
	}
	return normalized, nil
}

func validateViewKindBody(kind ViewKind, payload ViewPayload) error {
	set := 0
	if len(payload.Sections) > 0 {
		set++
	}
	if payload.Detail != nil {
		set++
	}
	if payload.Form != nil {
		set++
	}
	if payload.Grid != nil {
		set++
	}
	if set > 1 {
		return viewValidationError("payload", "must carry only the body for kind %q", kind)
	}
	switch kind {
	case ViewKindList:
		if payload.Detail != nil || payload.Form != nil || payload.Grid != nil {
			return viewValidationError("payload", "list kind cannot carry detail, form, or grid")
		}
	case ViewKindDetail:
		if payload.Detail == nil {
			return viewValidationError("detail", "is required for detail kind")
		}
	case ViewKindForm:
		if payload.Form == nil {
			return viewValidationError("form", "is required for form kind")
		}
	case ViewKindGrid:
		if payload.Grid == nil {
			return viewValidationError("grid", "is required for grid kind")
		}
	default:
		return &UnknownViewKindError{Kind: kind}
	}
	return nil
}

func validateViewFields(payload ViewPayload) error {
	if err := validateListFields(payload.Sections); err != nil {
		return err
	}
	if payload.Detail != nil {
		if err := validateRowActions("detail.actions", payload.Detail.Actions); err != nil {
			return err
		}
	}
	if err := validateFormFields(payload.Form); err != nil {
		return err
	}
	return validateGridFields(payload.Grid)
}

func validateListFields(sections []Section) error {
	for sectionIndex, section := range sections {
		if err := validateViewText(fmt.Sprintf("sections[%d].title", sectionIndex), section.Title, false); err != nil {
			return err
		}
		for rowIndex, row := range section.Rows {
			path := fmt.Sprintf("sections[%d].rows[%d]", sectionIndex, rowIndex)
			if strings.TrimSpace(row.ID) == "" {
				return viewValidationError(path+".id", "is required")
			}
			if err := validateViewText(path+".title", row.Title, true); err != nil {
				return err
			}
			if err := validateViewText(path+".subtitle", row.Subtitle, false); err != nil {
				return err
			}
			if err := validateBadge(path+".badge", row.Badge); err != nil {
				return err
			}
			if err := validateRowActions(path+".actions", row.Actions); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFormFields(form *FormBody) error {
	if form == nil {
		return nil
	}
	for index, field := range form.Fields {
		path := fmt.Sprintf("form.fields[%d]", index)
		if err := validateFormField(path, field); err != nil {
			return err
		}
	}
	if form.Submit != nil {
		if err := validateRowAction("form.submit", *form.Submit); err != nil {
			return err
		}
	}
	return nil
}

func validateGridFields(grid *GridBody) error {
	if grid == nil {
		return nil
	}
	for sectionIndex, section := range grid.Sections {
		for tileIndex, tile := range section.Tiles {
			path := fmt.Sprintf("grid.sections[%d].tiles[%d]", sectionIndex, tileIndex)
			if strings.TrimSpace(tile.ID) == "" {
				return viewValidationError(path+".id", "is required")
			}
			if err := validateViewText(path+".title", tile.Title, true); err != nil {
				return err
			}
			if err := validateImage(path+".image", tile.Image); err != nil {
				return err
			}
			if err := validateBadge(path+".badge", tile.Badge); err != nil {
				return err
			}
			if err := validateRowActions(path+".actions", tile.Actions); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateViewText(path, value string, required bool) error {
	if required && strings.TrimSpace(value) == "" {
		return viewValidationError(path, "is required")
	}
	if len(value) > MaxViewTextBytes {
		return viewValidationError(path, "is %d bytes; maximum is %d", len(value), MaxViewTextBytes)
	}
	return nil
}

func validateFormField(path string, field FormField) error {
	if strings.TrimSpace(field.ID) == "" {
		return viewValidationError(path+".id", "is required")
	}
	if err := validateViewText(path+".label", field.Label, true); err != nil {
		return err
	}
	if err := validateViewText(path+".placeholder", field.Placeholder, false); err != nil {
		return err
	}
	if err := validateViewText(path+".error", field.Error, false); err != nil {
		return err
	}
	if err := validateViewText(path+".empty_hint", field.EmptyHint, false); err != nil {
		return err
	}
	switch field.Type {
	case "text", string(ArgumentTypePassword), "textarea", "checkbox", "file":
		if len(field.Options) > 0 {
			return viewValidationError(path+".options", "is only valid for dropdown fields")
		}
	case "dropdown":
		for optionIndex, option := range field.Options {
			if err := validateViewText(
				fmt.Sprintf("%s.options[%d]", path, optionIndex), option, true,
			); err != nil {
				return err
			}
		}
	default:
		return viewValidationError(path+".type", "unknown field type %q", field.Type)
	}
	return nil
}

func validateRowActions(path string, actions []RowAction) error {
	for index, action := range actions {
		if err := validateRowAction(fmt.Sprintf("%s[%d]", path, index), action); err != nil {
			return err
		}
	}
	return nil
}

func validateRowAction(path string, action RowAction) error {
	set := 0
	if action.Action != nil {
		set++
	}
	if strings.TrimSpace(action.Handler) != "" {
		set++
	}
	if action.SubmitForm {
		set++
	}
	if set != 1 {
		return viewValidationError(path, "exactly one of action, handler, or submit_form is required")
	}
	if action.Destructive && action.Confirmation == nil {
		return viewValidationError(path+".confirmation", "is required for destructive actions")
	}
	if action.Action != nil {
		if err := validateAction(CommandID(path), *action.Action); err != nil {
			return viewValidationError(path+".action", "%v", err)
		}
	}
	return nil
}

func validateBadge(path string, badge *ViewBadge) error {
	if badge == nil {
		return nil
	}
	if _, ok := validBadgeTones[badge.Tone]; !ok {
		return viewValidationError(path+".tone", "unknown badge tone %q", badge.Tone)
	}
	return nil
}

func validateImage(path string, image Image) error {
	set := 0
	for _, value := range []string{image.URL, image.Token, image.Emoji} {
		if strings.TrimSpace(value) != "" {
			set++
		}
	}
	if set != 1 {
		return viewValidationError(path, "exactly one of url, token, or emoji is required")
	}
	return nil
}

func truncateViewDetails(payload *ViewPayload) {
	truncateDetail(payload.Detail)
	for sectionIndex := range payload.Sections {
		for rowIndex := range payload.Sections[sectionIndex].Rows {
			truncateDetail(payload.Sections[sectionIndex].Rows[rowIndex].Detail)
		}
	}
}

func truncateDetail(detail *DetailBody) {
	if detail == nil || len(detail.Markdown) <= MaxDetailMarkdown {
		return
	}
	limit := MaxDetailMarkdown - len(DetailTruncationMark)
	for limit > 0 && !utf8.ValidString(detail.Markdown[:limit]) {
		limit--
	}
	detail.Markdown = detail.Markdown[:limit] + DetailTruncationMark
}

func applyViewMountCap(payload *ViewPayload) {
	total := 0
	for _, section := range payload.Sections {
		total += len(section.Rows)
	}
	if total <= MaxViewMountRows {
		return
	}
	remaining := MaxViewMountRows
	for index := range payload.Sections {
		rows := payload.Sections[index].Rows
		if remaining >= len(rows) {
			remaining -= len(rows)
			continue
		}
		payload.Sections[index].Rows = append([]Row(nil), rows[:remaining]...)
		remaining = 0
	}
	payload.Chrome = ensureViewChrome(payload.Chrome)
	payload.Chrome.Pagination = &Pagination{HasMore: true, PageSize: MaxViewMountRows}
	if payload.Empty == nil {
		payload.Empty = &EmptyState{}
	}
	payload.Empty.Hint = viewOverflowMessage(MaxViewMountRows, total)
}

func viewOverflowMessage(showing, total int) string {
	return fmt.Sprintf("showing %d of %d", showing, total)
}

func ensureViewChrome(chrome *ViewChrome) *ViewChrome {
	if chrome != nil {
		return chrome
	}
	return &ViewChrome{}
}

func countViewComponents(payload ViewPayload) int {
	count := len(payload.Chips)
	for _, section := range payload.Sections {
		count++
		for _, row := range section.Rows {
			count += 1 + len(row.Actions)
			if row.Detail != nil {
				count += len(row.Detail.Metadata) + len(row.Detail.Actions)
			}
		}
	}
	if payload.Detail != nil {
		count += 1 + len(payload.Detail.Metadata) + len(payload.Detail.Actions)
	}
	if payload.Form != nil {
		count += 1 + len(payload.Form.Fields)
		if payload.Form.Submit != nil {
			count++
		}
	}
	if payload.Grid != nil {
		for _, section := range payload.Grid.Sections {
			count++
			for _, tile := range section.Tiles {
				count += 1 + len(tile.Actions)
			}
		}
	}
	return count
}

func viewValidationError(path, format string, arguments ...any) error {
	return &ViewValidationError{Path: path, Message: fmt.Sprintf(format, arguments...)}
}
