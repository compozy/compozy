package core

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// OpenAPIDocument caches the release contract across transports.
type OpenAPIDocument struct {
	load func() (openAPIDocumentBytes, error)
}

type openAPIDocumentBytes struct {
	data       []byte
	compressed []byte
	etag       string
}

func NewOpenAPIDocument(render func() ([]byte, error)) *OpenAPIDocument {
	return &OpenAPIDocument{load: sync.OnceValues(func() (openAPIDocumentBytes, error) {
		return renderOpenAPIDocument(render)
	})}
}

func renderOpenAPIDocument(render func() ([]byte, error)) (openAPIDocumentBytes, error) {
	if render == nil {
		return openAPIDocumentBytes{}, errors.New("OpenAPI document provider is not configured")
	}
	data, err := render()
	if err != nil {
		return openAPIDocumentBytes{}, err
	}
	hash := sha256.Sum256(data)
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return openAPIDocumentBytes{}, errors.Join(err, writer.Close())
	}
	if err := writer.Close(); err != nil {
		return openAPIDocumentBytes{}, err
	}
	return openAPIDocumentBytes{
		data:       data,
		compressed: compressed.Bytes(),
		etag:       `"` + hex.EncodeToString(hash[:]) + `"`,
	}, nil
}

func (h *BaseHandlers) GetOpenAPIDocument(c *gin.Context) {
	if h.OpenAPI == nil || h.OpenAPI.load == nil {
		RespondError(c, http.StatusInternalServerError, errors.New("OpenAPI document provider is not configured"), true)
		return
	}
	document, err := h.OpenAPI.load()
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, true)
		return
	}
	c.Header("ETag", document.etag)
	c.Header("Cache-Control", "no-cache")
	c.Writer.Header().Add("Vary", "Accept-Encoding")
	for candidate := range strings.SplitSeq(c.GetHeader("If-None-Match"), ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || strings.TrimPrefix(candidate, "W/") == document.etag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	data := document.data
	if acceptsGzip(c.GetHeader("Accept-Encoding")) {
		c.Header("Content-Encoding", "gzip")
		data = document.compressed
	}
	c.Data(http.StatusOK, "application/json", data)
}

func acceptsGzip(value string) bool {
	for encoding := range strings.SplitSeq(value, ",") {
		parts := strings.Split(encoding, ";")
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "gzip") {
			continue
		}
		quality := 1.0
		for _, parameter := range parts[1:] {
			key, value, ok := strings.Cut(strings.TrimSpace(parameter), "=")
			if ok && strings.EqualFold(key, "q") {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return false
				}
				quality = parsed
			}
		}
		return quality > 0 && quality <= 1
	}
	return false
}
