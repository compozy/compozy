package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	response, err := c.doRequestWithReaderAndClient(ctx, clientReaderRequest{
		Method:      http.MethodPost,
		Path:        path,
		Body:        body,
		ContentType: contentType,
		Credentials: agentidentity.Credentials{},
		Client:      c.httpClient,
	})
	bodyErr := body.Close()
	if err != nil || bodyErr != nil {
		return SessionAttachmentRecord{}, cleanupSessionAttachmentUpload(filePath, err, bodyErr, response)
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

func cleanupSessionAttachmentUpload(
	filePath string,
	requestErr error,
	bodyErr error,
	response *http.Response,
) error {
	var responseDrainErr, responseCloseErr error
	if response != nil && response.Body != nil {
		responseDrainErr = discardResponseBodyBounded(response.Body)
		responseCloseErr = response.Body.Close()
	}
	return errors.Join(
		requestErr,
		wrapSessionAttachmentBodyError(filePath, bodyErr),
		wrapSessionAttachmentResponseDrainError(responseDrainErr),
		wrapSessionAttachmentResponseCloseError(responseCloseErr),
	)
}

type sessionAttachmentMultipartBody struct {
	reader *io.PipeReader
	done   <-chan error
	once   sync.Once
	err    error
}

func (b *sessionAttachmentMultipartBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *sessionAttachmentMultipartBody) Close() error {
	b.once.Do(func() {
		readerErr := b.reader.Close()
		producerErr := <-b.done
		b.err = errors.Join(readerErr, producerErr)
	})
	return b.err
}

func sessionAttachmentMultipart(
	filePath string,
) (body *sessionAttachmentMultipartBody, contentType string, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("cli: open session attachment %q: %w", filePath, err)
	}

	info, err := file.Stat()
	if err != nil {
		closeErr := file.Close()
		return nil, "", errors.Join(
			fmt.Errorf("cli: stat session attachment %q: %w", filePath, err),
			wrapSessionAttachmentFileCloseError(filePath, closeErr),
		)
	}
	if !info.Mode().IsRegular() {
		closeErr := file.Close()
		return nil, "", errors.Join(
			fmt.Errorf("cli: session attachment %q is not a regular file", filePath),
			wrapSessionAttachmentFileCloseError(filePath, closeErr),
		)
	}
	name := filepath.Base(filePath)
	if strings.TrimSpace(name) == "" || name == "." || name == string(filepath.Separator) {
		closeErr := file.Close()
		return nil, "", errors.Join(
			fmt.Errorf("cli: session attachment %q has no file name", filePath),
			wrapSessionAttachmentFileCloseError(filePath, closeErr),
		)
	}

	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan error, 1)
	go func() {
		producerErr := writeSessionAttachmentMultipart(filePath, name, file, writer)
		pipeErr := pipeWriter.CloseWithError(producerErr)
		done <- errors.Join(producerErr, pipeErr)
		close(done)
	}()
	return &sessionAttachmentMultipartBody{reader: reader, done: done}, writer.FormDataContentType(), nil
}

func writeSessionAttachmentMultipart(
	filePath string,
	name string,
	file *os.File,
	writer *multipart.Writer,
) error {
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		return errors.Join(
			fmt.Errorf("cli: create session attachment multipart field: %w", err),
			wrapSessionAttachmentMultipartCloseError(writer.Close()),
			wrapSessionAttachmentFileCloseError(filePath, file.Close()),
		)
	}
	_, copyErr := io.Copy(part, file)
	return errors.Join(
		wrapSessionAttachmentReadError(filePath, copyErr),
		wrapSessionAttachmentMultipartCloseError(writer.Close()),
		wrapSessionAttachmentFileCloseError(filePath, file.Close()),
	)
}

func wrapSessionAttachmentReadError(filePath string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: read session attachment %q: %w", filePath, err)
}

func wrapSessionAttachmentMultipartCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: close session attachment multipart writer: %w", err)
}

func wrapSessionAttachmentFileCloseError(filePath string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: close session attachment %q: %w", filePath, err)
}

func wrapSessionAttachmentBodyError(filePath string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: close session attachment upload %q: %w", filePath, err)
}

func wrapSessionAttachmentResponseCloseError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: close failed attachment upload response: %w", err)
}

func wrapSessionAttachmentResponseDrainError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cli: drain failed attachment upload response: %w", err)
}
