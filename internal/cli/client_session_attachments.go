package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
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
		Method:         http.MethodPost,
		Path:           path,
		Body:           body,
		ContentType:    contentType,
		ContentLength:  body.contentLength,
		ExpectContinue: true,
		Credentials:    agentidentity.Credentials{},
		Client:         c.httpClient,
	})
	bodyErr := body.Wait()
	if err != nil {
		return SessionAttachmentRecord{}, cleanupSessionAttachmentUpload(filePath, err, bodyErr, response)
	}
	if response == nil {
		return SessionAttachmentRecord{}, cleanupSessionAttachmentUpload(
			filePath,
			errors.New("cli: session attachment upload returned no response"),
			bodyErr,
			nil,
		)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return SessionAttachmentRecord{}, sessionAttachmentUploadRejection(filePath, bodyErr, response)
	}
	if bodyErr != nil {
		return SessionAttachmentRecord{}, cleanupSessionAttachmentUpload(filePath, nil, bodyErr, response)
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

func sessionAttachmentUploadRejection(
	filePath string,
	bodyErr error,
	response *http.Response,
) error {
	apiErr := readAPIError(response)
	responseCloseErr := response.Body.Close()
	if errorTreeOnlyMatches(bodyErr, io.ErrClosedPipe) {
		bodyErr = nil
	}
	return errors.Join(
		apiErr,
		wrapSessionAttachmentBodyError(filePath, bodyErr),
		wrapSessionAttachmentResponseCloseError(responseCloseErr),
	)
}

func errorTreeOnlyMatches(err error, target error) bool {
	if err == nil || target == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !errorTreeOnlyMatches(child, target) {
				return false
			}
		}
		return true
	}
	if wrapped, ok := err.(interface{ Unwrap() error }); ok {
		return errorTreeOnlyMatches(wrapped.Unwrap(), target)
	}
	return errors.Is(err, target)
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
	reader        *io.PipeReader
	done          <-chan error
	contentLength int64
	once          sync.Once
	err           error
}

func (b *sessionAttachmentMultipartBody) Read(p []byte) (int, error) {
	return b.reader.Read(p)
}

func (b *sessionAttachmentMultipartBody) Close() error {
	b.finish()
	// net/http may close a streaming request as soon as the daemon rejects it.
	// Returning that expected producer interruption here makes Client.Do discard
	// the daemon's useful response. Wait reports the retained error to the owner.
	return nil
}

func (b *sessionAttachmentMultipartBody) Wait() error {
	b.finish()
	return b.err
}

func (b *sessionAttachmentMultipartBody) finish() {
	b.once.Do(func() {
		readerErr := b.reader.Close()
		producerErr := <-b.done
		b.err = errors.Join(readerErr, producerErr)
	})
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

	boundary, contentLength, err := sessionAttachmentMultipartMetadata(name, info.Size())
	if err != nil {
		closeErr := file.Close()
		return nil, "", errors.Join(err, wrapSessionAttachmentFileCloseError(filePath, closeErr))
	}
	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	if err := writer.SetBoundary(boundary); err != nil {
		readerCloseErr := reader.Close()
		writerCloseErr := pipeWriter.Close()
		fileCloseErr := file.Close()
		return nil, "", errors.Join(
			fmt.Errorf("cli: set session attachment multipart boundary: %w", err),
			readerCloseErr,
			writerCloseErr,
			wrapSessionAttachmentFileCloseError(filePath, fileCloseErr),
		)
	}
	done := make(chan error, 1)
	go func() {
		producerErr := writeSessionAttachmentMultipart(filePath, name, file, writer)
		pipeErr := pipeWriter.CloseWithError(producerErr)
		done <- errors.Join(producerErr, pipeErr)
		close(done)
	}()
	return &sessionAttachmentMultipartBody{
		reader:        reader,
		done:          done,
		contentLength: contentLength,
	}, writer.FormDataContentType(), nil
}

type sessionAttachmentByteCounter struct {
	bytes int64
}

func (c *sessionAttachmentByteCounter) Write(data []byte) (int, error) {
	if int64(len(data)) > math.MaxInt64-c.bytes {
		return 0, errors.New("cli: session attachment multipart length overflows int64")
	}
	c.bytes += int64(len(data))
	return len(data), nil
}

func sessionAttachmentMultipartMetadata(name string, fileBytes int64) (string, int64, error) {
	if fileBytes < 0 {
		return "", 0, fmt.Errorf("cli: session attachment has negative size %d", fileBytes)
	}
	counter := &sessionAttachmentByteCounter{}
	writer := multipart.NewWriter(counter)
	if _, err := writer.CreateFormFile("file", name); err != nil {
		return "", 0, errors.Join(
			fmt.Errorf("cli: size session attachment multipart field: %w", err),
			wrapSessionAttachmentMultipartCloseError(writer.Close()),
		)
	}
	if err := writer.Close(); err != nil {
		return "", 0, wrapSessionAttachmentMultipartCloseError(err)
	}
	if fileBytes > math.MaxInt64-counter.bytes {
		return "", 0, errors.New("cli: session attachment multipart length overflows int64")
	}
	return writer.Boundary(), counter.bytes + fileBytes, nil
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
