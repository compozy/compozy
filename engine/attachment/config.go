package attachment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

// baseAttachment holds fields shared by all attachments.
type baseAttachment struct {
	NameStr string         `json:"name,omitempty" yaml:"name,omitempty" mapstructure:"name,omitempty"`
	MIME    string         `json:"mime,omitempty" yaml:"mime,omitempty" mapstructure:"mime,omitempty"`
	MetaMap map[string]any `json:"meta,omitempty" yaml:"meta,omitempty" mapstructure:"meta,omitempty"`
}

func (b *baseAttachment) Name() string         { return b.NameStr }
func (b *baseAttachment) Meta() map[string]any { return b.MetaMap }

// Concrete attachment types with pluralized source support where applicable.

type ImageAttachment struct {
	baseAttachment
	Source Source   `json:"-"               yaml:"-"`
	URL    string   `json:"url,omitempty"   yaml:"url,omitempty"   mapstructure:"url,omitempty"`
	Path   string   `json:"path,omitempty"  yaml:"path,omitempty"  mapstructure:"path,omitempty"`
	URLs   []string `json:"urls,omitempty"  yaml:"urls,omitempty"  mapstructure:"urls,omitempty"`
	Paths  []string `json:"paths,omitempty" yaml:"paths,omitempty" mapstructure:"paths,omitempty"`
}

func (a *ImageAttachment) Type() Type { return TypeImage }
func (a *ImageAttachment) Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, cwd)
}

// MarshalJSON adds the discriminator field for correct round-trip encoding.
func (a *ImageAttachment) MarshalJSON() ([]byte, error) {
	type alias ImageAttachment
	return marshalAttachmentWithType(alias(*a), string(TypeImage))
}

type PDFAttachment struct {
	baseAttachment
	Source   Source   `json:"-"                   yaml:"-"`
	URL      string   `json:"url,omitempty"       yaml:"url,omitempty"       mapstructure:"url,omitempty"`
	Path     string   `json:"path,omitempty"      yaml:"path,omitempty"      mapstructure:"path,omitempty"`
	URLs     []string `json:"urls,omitempty"      yaml:"urls,omitempty"      mapstructure:"urls,omitempty"`
	Paths    []string `json:"paths,omitempty"     yaml:"paths,omitempty"     mapstructure:"paths,omitempty"`
	MaxPages *int     `json:"max_pages,omitempty" yaml:"max_pages,omitempty" mapstructure:"max_pages,omitempty"`
}

func (a *PDFAttachment) Type() Type { return TypePDF }
func (a *PDFAttachment) Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, cwd)
}

func (a *PDFAttachment) MarshalJSON() ([]byte, error) {
	type alias PDFAttachment
	return marshalAttachmentWithType(alias(*a), string(TypePDF))
}

type AudioAttachment struct {
	baseAttachment
	Source Source   `json:"-"               yaml:"-"`
	URL    string   `json:"url,omitempty"   yaml:"url,omitempty"   mapstructure:"url,omitempty"`
	Path   string   `json:"path,omitempty"  yaml:"path,omitempty"  mapstructure:"path,omitempty"`
	URLs   []string `json:"urls,omitempty"  yaml:"urls,omitempty"  mapstructure:"urls,omitempty"`
	Paths  []string `json:"paths,omitempty" yaml:"paths,omitempty" mapstructure:"paths,omitempty"`
}

func (a *AudioAttachment) Type() Type { return TypeAudio }
func (a *AudioAttachment) Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, cwd)
}

func (a *AudioAttachment) MarshalJSON() ([]byte, error) {
	type alias AudioAttachment
	return marshalAttachmentWithType(alias(*a), string(TypeAudio))
}

type VideoAttachment struct {
	baseAttachment
	Source Source   `json:"-"               yaml:"-"`
	URL    string   `json:"url,omitempty"   yaml:"url,omitempty"   mapstructure:"url,omitempty"`
	Path   string   `json:"path,omitempty"  yaml:"path,omitempty"  mapstructure:"path,omitempty"`
	URLs   []string `json:"urls,omitempty"  yaml:"urls,omitempty"  mapstructure:"urls,omitempty"`
	Paths  []string `json:"paths,omitempty" yaml:"paths,omitempty" mapstructure:"paths,omitempty"`
}

func (a *VideoAttachment) Type() Type { return TypeVideo }
func (a *VideoAttachment) Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, cwd)
}

func (a *VideoAttachment) MarshalJSON() ([]byte, error) {
	type alias VideoAttachment
	return marshalAttachmentWithType(alias(*a), string(TypeVideo))
}

type URLAttachment struct {
	baseAttachment
	URL string `json:"url" yaml:"url" mapstructure:"url"`
}

func (a *URLAttachment) Type() Type { return TypeURL }
func (a *URLAttachment) Resolve(ctx context.Context, _ *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, nil)
}

func (a *URLAttachment) MarshalJSON() ([]byte, error) {
	type alias URLAttachment
	return marshalAttachmentWithType(alias(*a), string(TypeURL))
}

type FileAttachment struct {
	baseAttachment
	Path string `json:"path" yaml:"path" mapstructure:"path"`
}

func (a *FileAttachment) Type() Type { return TypeFile }
func (a *FileAttachment) Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error) {
	return Resolve(ctx, a, cwd)
}

func (a *FileAttachment) MarshalJSON() ([]byte, error) {
	type alias FileAttachment
	return marshalAttachmentWithType(alias(*a), string(TypeFile))
}

// Attachments is a slice of polymorphic Attachment values with custom decoding.
type Attachments []Attachment

// UnmarshalYAML (goccy/go-yaml compatible) decodes a sequence using a type discriminator.
func (as *Attachments) UnmarshalYAML(unmarshal func(any) error) error {
	var raw []map[string]any
	if err := unmarshal(&raw); err != nil {
		var empty any
		if unmarshalErr := unmarshal(&empty); unmarshalErr != nil {
			return fmt.Errorf("failed to unmarshal attachments: %w", err)
		}
		if empty == nil {
			*as = nil
			return nil
		}
		return fmt.Errorf("failed to unmarshal attachments: %w", err)
	}
	if len(raw) == 0 {
		*as = nil
		return nil
	}
	items := make([]Attachment, 0, len(raw))
	for i, m := range raw {
		tval, ok := m["type"].(string)
		if !ok {
			return fmt.Errorf("attachment %d: missing or invalid type", i)
		}
		tp, err := normalizeType(tval)
		if err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		att, err := newForType(tp)
		if err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		b, err := json.Marshal(m)
		if err != nil {
			return fmt.Errorf("attachment %d: marshal failed: %w", i, err)
		}
		if err := json.Unmarshal(b, att); err != nil {
			return fmt.Errorf("attachment %d: decode failed: %w", i, err)
		}
		if err := validateAndSetSource(att); err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		items = append(items, att)
	}
	*as = items
	return nil
}

// UnmarshalJSON decodes a JSON array of attachments with a type discriminator.
func (as *Attachments) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err != nil {
		if string(data) == "null" || len(data) == 0 {
			*as = nil
			return nil
		}
		return fmt.Errorf("failed to unmarshal attachments: %w", err)
	}
	if len(arr) == 0 {
		*as = nil
		return nil
	}
	items := make([]Attachment, 0, len(arr))
	for i, raw := range arr {
		var td struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &td); err != nil {
			return fmt.Errorf("attachment %d: missing or invalid type: %w", i, err)
		}
		tp, err := normalizeType(td.Type)
		if err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		att, err := newForType(tp)
		if err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		if err := json.Unmarshal(raw, att); err != nil {
			return fmt.Errorf("attachment %d: decode failed: %w", i, err)
		}
		if err := validateAndSetSource(att); err != nil {
			return fmt.Errorf("attachment %d: %w", i, err)
		}
		items = append(items, att)
	}
	*as = items
	return nil
}

func normalizeType(v string) (Type, error) {
	if v == "" {
		return "", errors.New("type is required")
	}
	t := strings.ToLower(strings.TrimSpace(v))
	if t == "document" {
		t = string(TypePDF)
	}
	switch Type(t) {
	case TypeImage, TypeVideo, TypeAudio, TypePDF, TypeFile, TypeURL:
		return Type(t), nil
	default:
		return "", fmt.Errorf("unknown attachment type: %s", v)
	}
}

func newForType(t Type) (Attachment, error) {
	switch t {
	case TypeImage:
		return &ImageAttachment{}, nil
	case TypePDF:
		return &PDFAttachment{}, nil
	case TypeAudio:
		return &AudioAttachment{}, nil
	case TypeVideo:
		return &VideoAttachment{}, nil
	case TypeURL:
		return &URLAttachment{}, nil
	case TypeFile:
		return &FileAttachment{}, nil
	default:
		return nil, fmt.Errorf("unsupported attachment type: %s", t)
	}
}

func validateAndSetSource(att Attachment) error {
	switch a := att.(type) {
	case *ImageAttachment:
		src, err := validateMultiSource("image", a.URL, a.Path, a.URLs, a.Paths)
		if err != nil {
			return err
		}
		a.Source = src
		return nil
	case *PDFAttachment:
		src, err := validateMultiSource("pdf", a.URL, a.Path, a.URLs, a.Paths)
		if err != nil {
			return err
		}
		a.Source = src
		return nil
	case *AudioAttachment:
		src, err := validateMultiSource("audio", a.URL, a.Path, a.URLs, a.Paths)
		if err != nil {
			return err
		}
		a.Source = src
		return nil
	case *VideoAttachment:
		src, err := validateMultiSource("video", a.URL, a.Path, a.URLs, a.Paths)
		if err != nil {
			return err
		}
		a.Source = src
		return nil
	case *URLAttachment:
		if a.URL == "" {
			return errors.New("url attachment requires 'url'")
		}
		return nil
	case *FileAttachment:
		if a.Path == "" {
			return errors.New("file attachment requires 'path'")
		}
		return nil
	default:
		return fmt.Errorf("unknown attachment concrete type")
	}
}

func validateMultiSource(kind string, url string, path string, urls []string, paths []string) (Source, error) {
	provided := countProvidedSources(url, path, urls, paths)
	if len(provided) == 0 {
		return "", fmt.Errorf("%s attachment requires exactly one of 'url', 'path', 'urls', or 'paths'", kind)
	}
	if len(provided) > 1 {
		return "", fmt.Errorf(
			"%s attachment must not specify multiple source fields (provided fields: %s). "+
				"Use only one of: url, path, urls, or paths",
			kind,
			formatProvidedValues(url, path, urls, paths),
		)
	}
	if strings.TrimSpace(url) != "" || len(normalizeList(urls)) > 0 {
		return SourceURL, nil
	}
	return SourcePath, nil
}

// marshalAttachmentWithType provides a common JSON encoding for attachments by
// injecting the `type` discriminator and embedding the concrete fields.
func marshalAttachmentWithType(att any, typeStr string) ([]byte, error) {
	b, err := json.Marshal(att)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m["type"] = typeStr
	return json.Marshal(m)
}

// countProvidedSources returns which of the mutually exclusive source fields
// are present after trimming and normalizing inputs.
func countProvidedSources(url string, path string, urls []string, paths []string) []string {
	provided := make([]string, 0, 4)
	if strings.TrimSpace(url) != "" {
		provided = append(provided, "url")
	}
	if strings.TrimSpace(path) != "" {
		provided = append(provided, "path")
	}
	if len(normalizeList(urls)) > 0 {
		provided = append(provided, "urls")
	}
	if len(normalizeList(paths)) > 0 {
		provided = append(provided, "paths")
	}
	return provided
}

func normalizeList(xs []string) []string {
	if len(xs) == 0 {
		return xs
	}
	out := make([]string, 0, len(xs))
	for _, v := range xs {
		s := strings.TrimSpace(v)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func formatProvidedValues(url string, path string, urls []string, paths []string) string {
	parts := make([]string, 0, 4)
	if strings.TrimSpace(url) != "" {
		parts = append(parts, fmt.Sprintf("url=%q", strings.TrimSpace(url)))
	}
	if strings.TrimSpace(path) != "" {
		parts = append(parts, fmt.Sprintf("path=%q", strings.TrimSpace(path)))
	}
	nURLs := normalizeList(urls)
	if len(nURLs) > 0 {
		parts = append(parts, fmt.Sprintf("urls=%v", nURLs))
	}
	nPaths := normalizeList(paths)
	if len(nPaths) > 0 {
		parts = append(parts, fmt.Sprintf("paths=%v", nPaths))
	}
	return strings.Join(parts, ", ")
}
