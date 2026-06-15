// Package europeana is the library behind the europeana command line:
// the HTTP client, request shaping, and typed data models for the Europeana
// cultural heritage API (https://api.europeana.eu).
//
// A public demo API key ("api2demo") is used by default; override it with the
// EUROPEANA_KEY environment variable. The Client paces requests, sets a real
// User-Agent, and retries transient failures (429 and 5xx).
package europeana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Host is the API hostname.
const Host = "api.europeana.eu"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	APIKey    string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://api.europeana.eu",
		APIKey:    "api2demo",
		UserAgent: "europeana-cli/0.1 (+https://github.com/tamnd/europeana-cli)",
		Rate:      500 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Item holds the public data about a single Europeana cultural heritage item.
type Item struct {
	ID       string `kit:"id" json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Creator  string `json:"creator,omitempty"`
	Provider string `json:"provider,omitempty"`
	Country  string `json:"country,omitempty"`
	Year     string `json:"year,omitempty"`
	Language string `json:"language,omitempty"`
	Preview  string `json:"preview,omitempty"`
	URL      string `json:"url,omitempty"`
}

// --- wire types ---

type wireSearchResp struct {
	Success      bool       `json:"success"`
	TotalResults int        `json:"totalResults"`
	ItemsCount   int        `json:"itemsCount"`
	Items        []wireItem `json:"items"`
}

type wireItem struct {
	ID           string   `json:"id"`
	Title        []string `json:"title"`
	Type         string   `json:"type"`
	DcCreator    []string `json:"dcCreator"`
	DataProvider []string `json:"dataProvider"`
	Country      []string `json:"country"`
	Year         []string `json:"year"`
	Language     []string `json:"language"`
	EdmPreview   string   `json:"edmPreview"`
	EdmIsShownBy string   `json:"edmIsShownBy"`
	Guid         string   `json:"guid"`
}

// Client talks to the Europeana API.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client with the given configuration.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Search searches Europeana items by query, with optional media type filter and pagination.
func (c *Client) Search(ctx context.Context, query, mediaType string, start, rows int) ([]Item, error) {
	if rows <= 0 {
		rows = 25
	}
	if start <= 0 {
		start = 1
	}

	params := url.Values{}
	params.Set("wskey", c.cfg.APIKey)
	params.Set("query", query)
	params.Set("rows", fmt.Sprintf("%d", rows))
	params.Set("start", fmt.Sprintf("%d", start))
	if mediaType != "" {
		params.Set("type", mediaType)
	}

	rawURL := c.cfg.BaseURL + "/record/v2/search.json?" + params.Encode()
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	var resp wireSearchResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse search: %w", err)
	}

	out := make([]Item, len(resp.Items))
	for i, w := range resp.Items {
		out[i] = flattenItem(w)
	}
	return out, nil
}

// GetRecord fetches a single Europeana record by its ID (e.g. /91619/SMVK_EM_objekt_1059045).
func (c *Client) GetRecord(ctx context.Context, id string) (*Item, error) {
	// id already has leading slash, e.g. /91619/SMVK_EM_objekt_1059045
	if !strings.HasPrefix(id, "/") {
		id = "/" + id
	}
	params := url.Values{}
	params.Set("wskey", c.cfg.APIKey)
	rawURL := fmt.Sprintf("%s/record/v2%s.json?%s", c.cfg.BaseURL, id, params.Encode())

	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, err
	}

	// The single-record response wraps item in an "object" field
	var resp struct {
		Success bool     `json:"success"`
		Object  wireItem `json:"object"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse record: %w", err)
	}

	item := flattenItem(resp.Object)
	return &item, nil
}

// get fetches a URL and returns the body, pacing and retrying as configured.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- flatten helpers ---

func join(ss []string) string {
	return strings.Join(ss, "; ")
}

func flattenItem(w wireItem) Item {
	preview := w.EdmPreview
	if preview == "" {
		preview = w.EdmIsShownBy
	}
	return Item{
		ID:       w.ID,
		Title:    join(w.Title),
		Type:     w.Type,
		Creator:  join(w.DcCreator),
		Provider: join(w.DataProvider),
		Country:  join(w.Country),
		Year:     join(w.Year),
		Language: join(w.Language),
		Preview:  preview,
		URL:      w.Guid,
	}
}
