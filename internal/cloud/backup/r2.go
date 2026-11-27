// SPDX-License-Identifier: FSL-1.1-MIT

package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

const maxArchiveSize = 32 << 20

var bucketPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

type R2Config struct {
	Endpoint, Bucket, AccessKey, SecretKey string
}

type R2 struct {
	endpoint string
	creds    aws.Credentials
	signer   *v4.Signer
	http     *http.Client
}

func NewR2(cfg R2Config) (*R2, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && u.Scheme != "http") || !bucketPattern.MatchString(cfg.Bucket) ||
		cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("R2 archive requires an endpoint, bucket and credentials")
	}
	return &R2{
		endpoint: strings.TrimRight(cfg.Endpoint, "/") + "/" + cfg.Bucket,
		creds:    aws.Credentials{AccessKeyID: cfg.AccessKey, SecretAccessKey: cfg.SecretKey},
		signer:   v4.NewSigner(),
		http: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func (r *R2) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	query := url.Values{"list-type": {"2"}, "prefix": {prefix}}
	for {
		data, err := r.request(ctx, http.MethodGet, "?"+query.Encode(), nil)
		if err != nil {
			return nil, err
		}
		var page struct {
			Contents              []struct{ Key string }
			IsTruncated           bool
			NextContinuationToken string
		}
		if err := xml.Unmarshal(data, &page); err != nil {
			return nil, errors.New("decode R2 archive listing")
		}
		for _, item := range page.Contents {
			keys = append(keys, item.Key)
		}
		if !page.IsTruncated {
			return keys, nil
		}
		if page.NextContinuationToken == "" || page.NextContinuationToken == query.Get("continuation-token") {
			return nil, errors.New("R2 listing did not advance")
		}
		query.Set("continuation-token", page.NextContinuationToken)
	}
}

func (r *R2) Read(ctx context.Context, key string) ([]byte, error) {
	return r.request(ctx, http.MethodGet, "/"+key, nil)
}

func (r *R2) Write(ctx context.Context, key string, body []byte) error {
	_, err := r.request(ctx, http.MethodPut, "/"+key, body)
	return err
}

func (r *R2) request(ctx context.Context, method, suffix string, body []byte) (_ []byte, retErr error) {
	request, err := http.NewRequestWithContext(ctx, method, r.endpoint+suffix, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("create R2 archive request")
	}
	if method == http.MethodPut {
		request.Header.Set("If-None-Match", "*")
		request.Header.Set("Content-Type", "application/json")
	}
	digest := sha256.Sum256(body)
	checksum := hex.EncodeToString(digest[:])
	request.Header.Set("X-Amz-Content-Sha256", checksum)
	if err := r.signer.SignHTTP(ctx, r.creds, request, checksum, "s3", "auto", time.Now()); err != nil {
		return nil, errors.New("sign R2 archive request")
	}
	response, err := r.http.Do(request)
	if err != nil {
		return nil, errors.New("R2 archive request failed")
	}
	defer func() {
		_, drainErr := io.Copy(io.Discard, response.Body)
		retErr = errors.Join(retErr, drainErr, response.Body.Close())
	}()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("R2 archive returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxArchiveSize+1))
	if err != nil || len(data) > maxArchiveSize {
		return nil, errors.New("read R2 archive")
	}
	return data, nil
}
