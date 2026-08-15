package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

func (c *daemonClient) UploadSessionAttachment(
	ctx context.Context,
	id string,
	filePath string,
) (record SessionAttachmentRecord, err error) {
	if strings.TrimSpace(filePath) == "" {
		return SessionAttachmentRecord{}, errors.New("cli: attachment file path is required")
	}

	path, err := c.sessionScopedPath(ctx, id, "/attachments")
	if err != nil {
		return SessionAttachmentRecord{}, err
	}
	body, contentType, err := sessionAttachmentMultipart(filePath)
	if err != nil {
		return SessionAttachmentRecord{}, err
	}
	response, err := c.doRequestWithReaderAndClient(
		ctx,
		http.MethodPost,
		path,
		nil,
		body,
		contentType,
		"",
		agentidentity.Credentials{},
		c.httpClient,
	)
	if err != nil {
		return SessionAttachmentRecord{}, err
	}
	defer mergeResponseBodyCloseError(&err, response, http.MethodPost, path)

	var result struct {
		Attachment SessionAttachmentRecord `json:"attachment"`
	}
	if decodeErr := c.decodeJSONResponse(ctx, http.MethodPost, path, response, &result); decodeErr != nil {
		return SessionAttachmentRecord{}, decodeErr
	}
	return result.Attachment, nil
}

func sessionAttachmentMultipart(filePath string) (body *bytes.Buffer, contentType string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("cli: open session attachment %q: %w", filePath, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			closeErr = fmt.Errorf("cli: close session attachment %q: %w", filePath, closeErr)
			if err == nil {
				err = closeErr
				return
			}
			err = errors.Join(err, closeErr)
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("cli: stat session attachment %q: %w", filePath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("cli: session attachment %q is not a regular file", filePath)
	}
	name := filepath.Base(filePath)
	if strings.TrimSpace(name) == "" || name == "." || name == string(filepath.Separator) {
		return nil, "", fmt.Errorf("cli: session attachment %q has no file name", filePath)
	}

	body = &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return nil, "", fmt.Errorf("cli: create session attachment multipart field: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", fmt.Errorf("cli: read session attachment %q: %w", filePath, err)
	}
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("cli: close session attachment multipart writer: %w", err)
	}
	return body, writer.FormDataContentType(), nil
}
