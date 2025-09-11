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
func (a *ImageAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("image resolver not implemented")
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
func (a *PDFAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("pdf resolver not implemented")
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
func (a *AudioAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("audio resolver not implemented")
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
func (a *VideoAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("video resolver not implemented")
}

type URLAttachment struct {
	baseAttachment
	URL string `json:"url" yaml:"url" mapstructure:"url"`
}

func (a *URLAttachment) Type() Type { return TypeURL }
func (a *URLAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("url resolver not implemented")
}

type FileAttachment struct {
	baseAttachment
	Path string `json:"path" yaml:"path" mapstructure:"path"`
}

func (a *FileAttachment) Type() Type { return TypeFile }
func (a *FileAttachment) Resolve(_ context.Context, _ *core.PathCWD) (Resolved, error) {
	return nil, fmt.Errorf("file resolver not implemented")
}

// Config contains a list of polymorphic attachments.
type Config struct {
	Attachments []Attachment `json:"attachments,omitempty" yaml:"attachments,omitempty" mapstructure:"attachments,omitempty"`
}

// UnmarshalYAML implements custom decoding to support polymorphic attachments using type discriminator.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	var dto struct {
		Attachments []map[string]any `yaml:"attachments"`
	}
	if err := unmarshal(&dto); err != nil {
		return err
	}
	if len(dto.Attachments) == 0 {
		c.Attachments = nil
		return nil
	}
	items := make([]Attachment, 0, len(dto.Attachments))
	for i, m := range dto.Attachments {
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
	c.Attachments = items
	return nil
}

// UnmarshalJSON implements custom decoding to support polymorphic attachments using type discriminator.
func (c *Config) UnmarshalJSON(data []byte) error {
	var root struct {
		Attachments []json.RawMessage `json:"attachments"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	if len(root.Attachments) == 0 {
		c.Attachments = nil
		return nil
	}
	items := make([]Attachment, 0, len(root.Attachments))
	for i, raw := range root.Attachments {
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
	c.Attachments = items
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
	u := strings.TrimSpace(url)
	p := strings.TrimSpace(path)
	urls = normalizeList(urls)
	paths = normalizeList(paths)
	provided := make([]string, 0, 4)
	if u != "" {
		provided = append(provided, "url")
	}
	if p != "" {
		provided = append(provided, "path")
	}
	if len(urls) > 0 {
		provided = append(provided, "urls")
	}
	if len(paths) > 0 {
		provided = append(provided, "paths")
	}
	if len(provided) == 0 {
		return "", fmt.Errorf("%s attachment requires exactly one of 'url', 'path', 'urls', or 'paths'", kind)
	}
	if len(provided) > 1 {
		return "", fmt.Errorf(
			"%s attachment must not specify multiple source fields (provided: %s)",
			kind,
			strings.Join(provided, ","),
		)
	}
	if u != "" || len(urls) > 0 {
		return SourceURL, nil
	}
	return SourcePath, nil
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
