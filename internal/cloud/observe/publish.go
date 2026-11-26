// SPDX-License-Identifier: FSL-1.1-MIT

package observe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/klauspost/compress/s2"
	"google.golang.org/protobuf/encoding/protowire"
)

type Publisher struct {
	endpoint, user, token, environment, instance string
	client                                       *http.Client
}

func NewPublisher(endpoint, user, token, environment, instance string) (*Publisher, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "https" && (u.Scheme != "http" || environment != "development")) || user == "" || token == "" {
		return nil, errors.New("invalid Grafana Cloud remote-write configuration")
	}
	return &Publisher{endpoint, user, token, environment, instance, newHTTPClient()}, nil
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

func (p *Publisher) Publish(ctx context.Context, at time.Time, samples []Sample) (retErr error) {
	var request []byte
	for _, sample := range samples {
		labels := Labels{"__name__": sample.Name, "env": p.environment}
		if p.instance != "" {
			labels["instance"] = p.instance
		}
		maps.Copy(labels, sample.Labels)
		var series []byte
		for _, name := range slices.Sorted(maps.Keys(labels)) {
			label := wireBytes(nil, 1, []byte(name))
			label = wireBytes(label, 2, []byte(labels[name]))
			series = wireBytes(series, 1, label)
		}
		point := protowire.AppendTag(nil, 1, protowire.Fixed64Type)
		point = protowire.AppendFixed64(point, math.Float64bits(sample.Value))
		point = protowire.AppendTag(point, 2, protowire.VarintType)
		point = protowire.AppendVarint(point, uint64(at.UnixMilli()))
		series = wireBytes(series, 2, point)
		request = wireBytes(request, 1, series)
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.endpoint,
		bytes.NewReader(s2.EncodeSnappy(nil, request)),
	)
	if err != nil {
		return errors.New("create Grafana Cloud remote-write request")
	}
	req.SetBasicAuth(p.user, p.token)
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return errors.New("send remote-write request to Grafana Cloud")
	}
	defer func() {
		_, err := io.Copy(io.Discard, resp.Body)
		retErr = errors.Join(retErr, err, resp.Body.Close())
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote-write request to Grafana Cloud returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func wireBytes(dst []byte, field protowire.Number, value []byte) []byte {
	return protowire.AppendBytes(protowire.AppendTag(dst, field, protowire.BytesType), value)
}
