package europeana_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/europeana-cli/europeana"
)

func newTestClient(ts *httptest.Server) *europeana.Client {
	cfg := europeana.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return europeana.NewClient(cfg)
}

// TestSearchReturnsItems checks that search results are parsed correctly.
func TestSearchReturnsItems(t *testing.T) {
	fixture := map[string]any{
		"success":      true,
		"totalResults": 66724310,
		"itemsCount":   2,
		"items": []any{
			map[string]any{
				"id":           "/91619/SMVK_EM_objekt_1059045",
				"title":        []any{"Wooden mask"},
				"type":         "IMAGE",
				"dcCreator":    []any{"Unknown Artist"},
				"dataProvider": []any{"Världskulturmuseerna"},
				"country":      []any{"Sweden"},
				"year":         []any{"1900"},
				"language":     []any{"sv"},
				"edmPreview":   "https://example.com/thumb.jpg",
				"guid":         "https://www.europeana.eu/item/91619/SMVK_EM_objekt_1059045",
			},
			map[string]any{
				"id":    "/2048128/photograph_ProvidedCHO_bibliotheque_de_Rennes_Metropole_22Res_1040_59",
				"title": []any{"Portrait of a woman"},
				"type":  "IMAGE",
			},
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(fixture)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Search(context.Background(), "mask", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	first := items[0]
	if first.ID != "/91619/SMVK_EM_objekt_1059045" {
		t.Errorf("ID = %q", first.ID)
	}
	if first.Title != "Wooden mask" {
		t.Errorf("Title = %q", first.Title)
	}
	if first.Type != "IMAGE" {
		t.Errorf("Type = %q", first.Type)
	}
	if first.Creator != "Unknown Artist" {
		t.Errorf("Creator = %q", first.Creator)
	}
	if first.Provider != "Världskulturmuseerna" {
		t.Errorf("Provider = %q", first.Provider)
	}
	if first.Country != "Sweden" {
		t.Errorf("Country = %q", first.Country)
	}
	if first.Year != "1900" {
		t.Errorf("Year = %q", first.Year)
	}
	if first.Preview != "https://example.com/thumb.jpg" {
		t.Errorf("Preview = %q", first.Preview)
	}
	if first.URL != "https://www.europeana.eu/item/91619/SMVK_EM_objekt_1059045" {
		t.Errorf("URL = %q", first.URL)
	}
}

// TestSearchTypeFilterSendsParam checks that --type is forwarded in the query.
func TestSearchTypeFilterSendsParam(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		resp := map[string]any{
			"success":      true,
			"totalResults": 0,
			"itemsCount":   0,
			"items":        []any{},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Search(context.Background(), "painting", "IMAGE", 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "type=IMAGE") {
		t.Errorf("query %q should contain type=IMAGE", gotQuery)
	}
	if !strings.Contains(gotQuery, "query=painting") {
		t.Errorf("query %q should contain query=painting", gotQuery)
	}
	if !strings.Contains(gotQuery, "rows=10") {
		t.Errorf("query %q should contain rows=10", gotQuery)
	}
}

// TestGetRecordReturnsSingleItem checks that a single record is parsed correctly.
func TestGetRecordReturnsSingleItem(t *testing.T) {
	fixture := map[string]any{
		"success": true,
		"object": map[string]any{
			"id":           "/91619/SMVK_EM_objekt_1059045",
			"title":        []any{"Wooden mask"},
			"type":         "IMAGE",
			"dcCreator":    []any{"Unknown Artist"},
			"dataProvider": []any{"Världskulturmuseerna"},
			"country":      []any{"Sweden"},
			"year":         []any{"1900"},
			"language":     []any{"sv"},
			"edmPreview":   "https://example.com/thumb.jpg",
			"edmIsShownBy": "https://example.com/full.jpg",
			"guid":         "https://www.europeana.eu/item/91619/SMVK_EM_objekt_1059045",
		},
	}

	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := json.Marshal(fixture)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	item, err := c.GetRecord(context.Background(), "/91619/SMVK_EM_objekt_1059045")
	if err != nil {
		t.Fatal(err)
	}
	if item == nil {
		t.Fatal("item is nil")
	}
	if item.ID != "/91619/SMVK_EM_objekt_1059045" {
		t.Errorf("ID = %q", item.ID)
	}
	if item.Title != "Wooden mask" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.Type != "IMAGE" {
		t.Errorf("Type = %q", item.Type)
	}
	// Check URL path was built correctly
	if !strings.HasSuffix(gotPath, ".json") {
		t.Errorf("path %q should end with .json", gotPath)
	}
	if !strings.Contains(gotPath, "91619") {
		t.Errorf("path %q should contain the record ID", gotPath)
	}
}

// TestRetryOn503 checks that the client retries on 503 and succeeds eventually.
func TestRetryOn503(t *testing.T) {
	var hits int
	fixture := map[string]any{
		"success":      true,
		"totalResults": 1,
		"itemsCount":   1,
		"items": []any{
			map[string]any{
				"id":    "/123/abc",
				"title": []any{"Test item"},
				"type":  "IMAGE",
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		b, _ := json.Marshal(fixture)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	cfg := europeana.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := europeana.NewClient(cfg)

	start := time.Now()
	items, err := c.Search(context.Background(), "test", "", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

// TestUserAgent checks that every request carries europeana-cli in User-Agent.
func TestUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		resp := map[string]any{
			"success":      true,
			"totalResults": 0,
			"itemsCount":   0,
			"items":        []any{},
		}
		b, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(b)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _ = c.Search(context.Background(), "test", "", 0, 0)

	if !strings.Contains(gotUA, "europeana-cli") {
		t.Errorf("User-Agent = %q, want it to contain europeana-cli", gotUA)
	}
}
