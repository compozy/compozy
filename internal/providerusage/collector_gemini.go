package providerusage

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type GeminiCollector struct{ collectorHTTP }

var _ Collector = (*GeminiCollector)(nil)

func NewGeminiCollector(options ...CollectorOption) *GeminiCollector {
	return &GeminiCollector{newCollectorHTTP("https://cloudcode-pa.googleapis.com", options)}
}
func (*GeminiCollector) ProviderID() string { return "gemini" }
func (c *GeminiCollector) credential(t Target) (collectorCredential, error) {
	home, err := collectorHomePath(t, "", ".gemini")
	if err != nil {
		return collectorCredential{}, err
	}
	var data struct {
		Access string `json:"access_token"`
		ID     string `json:"id_token"`
		Expiry int64  `json:"expiry_date"`
	}
	if err := readCollectorCredential(filepath.Join(home, "oauth_creds.json"), "Gemini", &data); err != nil {
		return collectorCredential{}, err
	}
	if data.Access == "" {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthMissing, "Gemini")
	}
	if expiredCollectorToken(data.Expiry/1000, c.now()) {
		return collectorCredential{}, NewUsageError(ErrorCodeAuthExpired, "Gemini")
	}
	value := collectorCredential{token: data.Access}
	if claims, err := readCollectorClaims(data.ID); err == nil {
		value.account = claims.Subject
		value.email = claims.Email
		value.domain = claims.Domain
	}
	return value, nil
}
func (c *GeminiCollector) Identity(_ context.Context, t Target, _ Intent) (CredentialIdentity, error) {
	value, err := c.credential(t)
	if err != nil {
		return CredentialIdentity{}, err
	}
	return value.identity(), nil
}
func (c *GeminiCollector) post(ctx context.Context, path, token, body string, out any) error {
	data, err := c.client().Do(ctx, http.MethodPost, path, bearerHeaders(token), strings.NewReader(body), "Gemini")
	if err != nil {
		return err
	}
	return decodeCollectorJSON(data, out, "Gemini")
}
func (c *GeminiCollector) Collect(ctx context.Context, t Target, _ Intent, cred CredentialIdentity) (Snapshot, error) {
	value, err := c.credential(t)
	if err != nil {
		return Snapshot{}, err
	}
	if err := checkedCollectorCredential(value, cred, "Gemini"); err != nil {
		return Snapshot{}, err
	}
	var assist struct {
		Project json.RawMessage `json:"cloudaicompanionProject"`
		Current *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"currentTier"`
		Paid *struct {
			Name string `json:"name"`
		} `json:"paidTier"`
	}
	if err := c.post(
		ctx,
		"/v1internal:loadCodeAssist",
		value.token,
		`{"metadata":{"ideType":"GEMINI_CLI","pluginType":"GEMINI"}}`,
		&assist,
	); err != nil {
		return Snapshot{}, err
	}
	snapshot := c.snapshot(t, Source{Kind: SourceKindOAuthHTTP, Label: "Gemini CLI OAuth"})
	snapshot.Account = &Account{Label: value.email}
	if assist.Current != nil {
		snapshot.Account.Plan = assist.Current.Name
		if snapshot.Account.Plan == "" {
			snapshot.Account.Plan = assist.Current.ID
		}
	}
	if assist.Paid != nil && assist.Paid.Name != "" {
		snapshot.Account.Plan = assist.Paid.Name
	}
	project, err := geminiProject(assist.Project)
	if err != nil {
		return snapshot, err
	}
	request := struct {
		Project string `json:"project,omitempty"`
	}{Project: project}
	body, err := json.Marshal(request)
	if err != nil {
		return snapshot, NewUsageError(ErrorCodeParseError, "Gemini")
	}
	var quota geminiQuota
	if err := c.post(ctx, "/v1internal:retrieveUserQuota", value.token, string(body), &quota); err != nil {
		if AsUsageError(err, "Gemini").Code == ErrorCodeAuthScope {
			err = NewUsageError(ErrorCodeProviderUnavailable, "Gemini")
		}
		snapshot.Status = AsUsageError(err, "Gemini").Status()
		snapshot.Error = AsUsageError(err, "Gemini")
		snapshot.MeasuredAt = nil
		return snapshot, err
	}
	windows, err := quota.windows()
	if err != nil {
		return snapshot, err
	}
	snapshot.Windows = windows
	snapshot.Confidence = ConfidencePercentOnly
	return snapshot, nil
}

type geminiQuota struct {
	Buckets []struct {
		Model     string   `json:"modelId"`
		Remaining *float64 `json:"remainingFraction"`
		Reset     string   `json:"resetTime"`
	} `json:"buckets"`
}

func (quota geminiQuota) windows() ([]Window, error) {
	var result []Window
	windows := map[string]Window{}
	for _, bucket := range quota.Buckets {
		slug := strings.Trim(collectorModelSlug.ReplaceAllString(strings.ToLower(bucket.Model), "-"), "-")
		if slug == "" || bucket.Remaining == nil || *bucket.Remaining > 1 || !validCollectorPercent(*bucket.Remaining) {
			return nil, NewUsageError(ErrorCodeParseError, "Gemini")
		}
		percent := (1 - *bucket.Remaining) * 100
		id := "model:" + slug
		if previous, exists := windows[id]; !exists || percent > *previous.UsedPercent {
			windows[id] = Window{
				ID:          id,
				Label:       bucket.Model,
				Kind:        WindowKindModel,
				UsedPercent: &percent,
				ResetsAt:    collectorReset(bucket.Reset),
			}
		}
	}
	if len(windows) == 0 {
		return nil, NewUsageError(ErrorCodeParseError, "Gemini")
	}
	for _, w := range windows {
		result = append(result, w)
	}
	slices.SortFunc(result, func(a, b Window) int { return cmp.Compare(a.ID, b.ID) })
	return result, nil
}

var collectorModelSlug = regexp.MustCompile(`[^a-z0-9]+`)

func geminiProject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var project string
	if err := json.Unmarshal(raw, &project); err == nil {
		return project, nil
	}
	var object struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", NewUsageError(ErrorCodeParseError, "Gemini")
	}
	return object.ID, nil
}
