package attachment

import (
	"encoding/json"
	"testing"

	yaml "github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testWrap struct {
	Attachments Attachments `yaml:"attachments" json:"attachments"`
}

func Test_Config_UnmarshalYAML_Polymorphic(t *testing.T) {
	t.Run("Should unmarshal mixed attachment types with alias normalization", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: image
    url: https://example.com/1.png
    name: img1
    meta:
      source: remote
  - type: document
    path: ./specs/doc.pdf
    name: spec
  - type: audio
    urls: ["https://a/1.mp3", "https://a/2.mp3"]
  - type: video
    paths: ["./videos/v1.mp4"]
  - type: url
    url: https://example.com
  - type: file
    path: ./bin/asset.bin
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.NoError(t, err)
		require.Len(t, cfg.Attachments, 6)

		img, ok := cfg.Attachments[0].(*ImageAttachment)
		require.True(t, ok)
		assert.Equal(t, TypeImage, img.Type())
		assert.Equal(t, SourceURL, img.Source)
		assert.Equal(t, "img1", img.Name())
		assert.Equal(t, "remote", img.Meta()["source"])

		pdf, ok := cfg.Attachments[1].(*PDFAttachment)
		require.True(t, ok)
		assert.Equal(t, TypePDF, pdf.Type())
		assert.Equal(t, SourcePath, pdf.Source)

		aud, ok := cfg.Attachments[2].(*AudioAttachment)
		require.True(t, ok)
		assert.Equal(t, SourceURL, aud.Source)

		vid, ok := cfg.Attachments[3].(*VideoAttachment)
		require.True(t, ok)
		assert.Equal(t, SourcePath, vid.Source)

		u, ok := cfg.Attachments[4].(*URLAttachment)
		require.True(t, ok)
		assert.Equal(t, TypeURL, u.Type())

		f, ok := cfg.Attachments[5].(*FileAttachment)
		require.True(t, ok)
		assert.Equal(t, TypeFile, f.Type())
	})

	t.Run("Should error when multiple sources are provided", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: image
    url: https://example.com/a.png
    path: ./local.png
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "must not specify multiple source fields")
		assert.ErrorContains(t, err, "provided: url,path")
	})

	t.Run("Should error when no sources are provided", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: image
    name: missing
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "requires exactly one of")
	})

	t.Run("Should error when urls/paths are only empty strings", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: image
    urls: [" ", "\t"]
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "requires exactly one of")
	})

	t.Run("Should normalize YAML alias 'document' to pdf", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: document
    path: ./doc.pdf
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.NoError(t, err)
		require.Len(t, cfg.Attachments, 1)
		_, ok := cfg.Attachments[0].(*PDFAttachment)
		require.True(t, ok)
	})

	t.Run("Should error for unknown YAML type", func(t *testing.T) {
		data := []byte(`
attachments:
  - type: unknown
    url: https://x
`)
		var cfg testWrap
		err := yaml.Unmarshal(data, &cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unknown attachment type")
	})
}

func Test_Config_UnmarshalJSON_Polymorphic(t *testing.T) {
	t.Run("Should unmarshal JSON with mixed types", func(t *testing.T) {
		raw := []byte(`{
  "attachments": [
    {"type":"image","paths":["./a.png"]},
    {"type":"pdf","urls":["https://x/spec.pdf"]},
    {"type":"url","url":"https://example.com"},
    {"type":"file","path":"./bin.bin"}
  ]
}`)
		var cfg testWrap
		err := json.Unmarshal(raw, &cfg)
		require.NoError(t, err)
		require.Len(t, cfg.Attachments, 4)
		_, ok := cfg.Attachments[0].(*ImageAttachment)
		require.True(t, ok)
		_, ok = cfg.Attachments[1].(*PDFAttachment)
		require.True(t, ok)
		_, ok = cfg.Attachments[2].(*URLAttachment)
		require.True(t, ok)
		_, ok = cfg.Attachments[3].(*FileAttachment)
		require.True(t, ok)
	})

	t.Run("Should error for unknown type", func(t *testing.T) {
		raw := []byte(`{"attachments":[{"type":"unknown"}]}`)
		var cfg testWrap
		err := json.Unmarshal(raw, &cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "unknown attachment type")
	})

	t.Run("Should normalize JSON alias 'document' to pdf", func(t *testing.T) {
		raw := []byte(`{"attachments":[{"type":"document","path":"./doc.pdf"}]}`)
		var cfg testWrap
		err := json.Unmarshal(raw, &cfg)
		require.NoError(t, err)
		require.Len(t, cfg.Attachments, 1)
		_, ok := cfg.Attachments[0].(*PDFAttachment)
		require.True(t, ok)
	})
}
