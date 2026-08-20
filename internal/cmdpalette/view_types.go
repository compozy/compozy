package cmdpalette

import "encoding/json"

type ViewKind string

const (
	ViewKindList   ViewKind = "list"
	ViewKindDetail ViewKind = "detail"
	ViewKindForm   ViewKind = "form"
	ViewKindGrid   ViewKind = "grid"
)

// ViewPayload is the frozen v1 view wire shared by declarative and programmable views.
type ViewPayload struct {
	View     string      `json:"view"`
	Chrome   *ViewChrome `json:"chrome,omitempty"`
	Sections []Section   `json:"sections,omitempty"`
	Chips    []Chip      `json:"chips,omitempty"`
	Empty    *EmptyState `json:"empty,omitempty"`
	Detail   *DetailBody `json:"detail,omitempty"`
	Form     *FormBody   `json:"form,omitempty"`
	Grid     *GridBody   `json:"grid,omitempty"`
}

type ViewChrome struct {
	IsLoading   bool        `json:"is_loading,omitempty"`
	SearchText  string      `json:"search_text,omitempty"`
	EventCount  int64       `json:"event_count,omitempty"`
	Placeholder string      `json:"search_placeholder,omitempty"`
	ThrottleMs  int         `json:"throttle_ms,omitempty"`
	Filtering   *bool       `json:"filtering,omitempty"`
	Complete    bool        `json:"complete,omitempty"`
	ActiveChip  string      `json:"active_chip,omitempty"`
	Columns     int         `json:"columns,omitempty"`
	Pagination  *Pagination `json:"pagination,omitempty"`
	OnSearch    string      `json:"on_search,omitempty"`
	OnChip      string      `json:"on_chip,omitempty"`
	OnSelection string      `json:"on_selection,omitempty"`
	OnLoadMore  string      `json:"on_load_more,omitempty"`
}

type Section struct {
	Title string `json:"title,omitempty"`
	Rows  []Row  `json:"rows"`
}

type Row struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Subtitle    string            `json:"subtitle,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Badge       *ViewBadge        `json:"badge,omitempty"`
	Keywords    []string          `json:"keywords,omitempty"`
	Accessories []string          `json:"accessories,omitempty"`
	Detail      *DetailBody       `json:"detail,omitempty"`
	Actions     []RowAction       `json:"actions,omitempty"`
	Requires    map[string]string `json:"requires,omitempty"`
	Fallback    string            `json:"fallback,omitempty"`
}

type RowAction struct {
	Title        string            `json:"title"`
	Icon         string            `json:"icon,omitempty"`
	Section      string            `json:"section,omitempty"`
	Primary      bool              `json:"primary,omitempty"`
	Destructive  bool              `json:"destructive,omitempty"`
	Confirmation *Confirmation     `json:"confirmation,omitempty"`
	Shortcut     string            `json:"shortcut,omitempty"`
	Action       *Action           `json:"action,omitempty"`
	Handler      string            `json:"handler,omitempty"`
	SubmitForm   bool              `json:"submit_form,omitempty"`
	Requires     map[string]string `json:"requires,omitempty"`
	Fallback     string            `json:"fallback,omitempty"`
}

type Chip struct {
	ID       string            `json:"id"`
	Label    string            `json:"label"`
	Count    *int              `json:"count,omitempty"`
	Requires map[string]string `json:"requires,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
}

type EmptyState struct {
	Title string `json:"title"`
	Hint  string `json:"hint,omitempty"`
	Icon  string `json:"icon,omitempty"`
}

type Pagination struct {
	HasMore  bool `json:"has_more"`
	PageSize int  `json:"page_size,omitempty"`
}

type DetailBody struct {
	IsLoading bool        `json:"is_loading,omitempty"`
	Markdown  string      `json:"markdown,omitempty"`
	Metadata  []MetaField `json:"metadata,omitempty"`
	Actions   []RowAction `json:"actions,omitempty"`
}

type FormBody struct {
	Fields   []FormField `json:"fields"`
	Submit   *RowAction  `json:"submit,omitempty"`
	OnSubmit string      `json:"on_submit,omitempty"`
}

type FormField struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Label       string            `json:"label"`
	Placeholder string            `json:"placeholder,omitempty"`
	Required    bool              `json:"required,omitempty"`
	Options     []string          `json:"options,omitempty"`
	Directories bool              `json:"directories,omitempty"`
	Default     any               `json:"default,omitempty"`
	Error       string            `json:"error,omitempty"`
	EmptyHint   string            `json:"empty_hint,omitempty"`
	OnChange    string            `json:"on_change,omitempty"`
	OnBlur      string            `json:"on_blur,omitempty"`
	EventCount  int64             `json:"event_count,omitempty"`
	Requires    map[string]string `json:"requires,omitempty"`
	Fallback    string            `json:"fallback,omitempty"`
}

type GridBody struct {
	Sections []GridSection `json:"sections"`
}

type GridSection struct {
	Title string     `json:"title,omitempty"`
	Tiles []GridTile `json:"tiles"`
}

type GridTile struct {
	ID       string            `json:"id"`
	Title    string            `json:"title"`
	Image    Image             `json:"image"`
	Badge    *ViewBadge        `json:"badge,omitempty"`
	Actions  []RowAction       `json:"actions,omitempty"`
	Requires map[string]string `json:"requires,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
}

type Effect struct {
	ID        string           `json:"id"`
	Toast     *ToastEffect     `json:"toast,omitempty"`
	Copy      *CopyEffect      `json:"copy,omitempty"`
	OpenURL   *OpenURLEffect   `json:"open_url,omitempty"`
	OpenApp   *OpenAppEffect   `json:"open_app,omitempty"`
	PickFiles *PickFilesEffect `json:"pick_files,omitempty"`
}

type ToastEffect struct {
	Tone    string `json:"tone"`
	Message string `json:"message"`
}

type CopyEffect struct {
	Content string `json:"content"`
}

type OpenURLEffect struct {
	URL string `json:"url"`
}

type OpenAppEffect struct {
	App string `json:"app"`
}

type PickFilesEffect struct {
	Directories bool `json:"directories,omitempty"`
}

type ViewPatch struct {
	ViewID string    `json:"view_id"`
	From   string    `json:"from"`
	To     string    `json:"to"`
	Ops    []PatchOp `json:"ops"`
}

type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value,omitempty"`
}

type ViewBadge struct {
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

type Image struct {
	URL   string `json:"url,omitempty"`
	Token string `json:"token,omitempty"`
	Emoji string `json:"emoji,omitempty"`
}

type MetaField struct {
	Label    string            `json:"label"`
	Value    string            `json:"value"`
	Requires map[string]string `json:"requires,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
}
