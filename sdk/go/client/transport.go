package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// UploadFile supplies an attachment; its Reader remains owned by the caller.
type UploadFile struct {
	Name        string
	ContentType string
	Reader      io.Reader
}

// Do sends a JSON request to a daemon-relative path and decodes its successful body.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	return c.do(ctx, method, path, nil, body, out)
}

func (c *Client) do(ctx context.Context, method, path string, headers http.Header, body, out any) error {
	return c.doContent(ctx, method, path, headers, body, out, "application/json", "application/json")
}

func (c *Client) doContent(
	ctx context.Context, method, path string, headers http.Header, body, out any,
	requestContentType, responseContentType string,
) (err error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	headers = headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if responseContentType != "" {
		headers.Set("Accept", responseContentType)
	}
	response, err := c.request(ctx, method, path, headers, body, requestContentType)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, drainClose(response.Body)) }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return responseError(response)
	}
	if out == nil || response.StatusCode == http.StatusNoContent {
		return nil
	}
	if responseContentType == "" || responseContentType == "application/json" || strings.HasSuffix(responseContentType, "+json") {
		err = json.NewDecoder(response.Body).Decode(out)
	} else {
		switch target := out.(type) {
		case *[]byte:
			*target, err = io.ReadAll(response.Body)
		case *string:
			var data []byte
			data, err = io.ReadAll(response.Body)
			*target = string(data)
		default:
			err = errors.New("non-JSON response requires a string or byte slice destination")
		}
	}
	if err != nil {
		return fmt.Errorf("compozy client: decode %s %s: %w", method, path, err)
	}
	return nil
}

func (c *Client) request(
	ctx context.Context, method, path string, headers http.Header, body any, contentType string,
) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "#") {
		return nil, errors.New("compozy client: request path must be daemon-relative and start with one slash")
	}
	reader, contentType, err := requestBody(body, contentType)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("compozy client: create request: %w", err)
	}
	request.Header = headers.Clone()
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	if reader != nil {
		request.Header.Set("Content-Type", contentType)
	}
	request.Header.Set("X-Compozy-Client-ID", c.clientID)
	if c.token != "" {
		request.Header.Set("Authorization", "Bearer "+c.token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		if ctx.Err() == nil {
			return nil, fmt.Errorf("compozy client: %s %s: %w: %w", method, path, ErrUnavailable, err)
		}
		return nil, fmt.Errorf("compozy client: %s %s: %w", method, path, err)
	}
	return response, nil
}

func requestBody(body any, contentType string) (io.Reader, string, error) {
	if body == nil {
		return nil, contentType, nil
	}
	if contentType == "multipart/form-data" {
		file, ok := body.(*UploadFile)
		if !ok || file == nil || file.Reader == nil || file.Name == "" {
			return nil, "", errors.New("compozy client: upload requires a file name and reader")
		}
		return multipartBody(file)
	}
	if contentType != "" && contentType != "application/json" {
		switch value := body.(type) {
		case io.Reader:
			return value, contentType, nil
		case []byte:
			return bytes.NewReader(value), contentType, nil
		case string:
			return strings.NewReader(value), contentType, nil
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("compozy client: encode request: %w", err)
	}
	return bytes.NewReader(data), "application/json", nil
}

func multipartBody(file *UploadFile) (io.Reader, string, error) {
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil || strings.ContainsAny(contentType, "\r\n") {
		return nil, "", errors.New("compozy client: invalid file content type")
	}
	var buffer bytes.Buffer
	writer := multipart.NewWriter(&buffer)
	headers := make(textproto.MIMEHeader)
	headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": file.Name}))
	headers.Set("Content-Type", contentType)
	if _, err := writer.CreatePart(headers); err != nil {
		return nil, "", fmt.Errorf("compozy client: create upload: %w", err)
	}
	prefix := bytes.Clone(buffer.Bytes())
	buffer.Reset()
	if err := writer.Close(); err != nil {
		return nil, "", fmt.Errorf("compozy client: finish upload: %w", err)
	}
	return io.MultiReader(bytes.NewReader(prefix), file.Reader, bytes.NewReader(buffer.Bytes())), writer.FormDataContentType(), nil
}

func drainClose(body io.ReadCloser) error {
	_, drainErr := io.Copy(io.Discard, body)
	return errors.Join(drainErr, body.Close())
}
