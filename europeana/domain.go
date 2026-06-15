package europeana

import (
	"context"
	"os"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

func init() { kit.Register(Domain{}) }

// Domain is the europeana driver.
type Domain struct{}

// Info describes the scheme, hostnames, and binary identity.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "europeana",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "europeana",
			Short:  "A command line for the Europeana cultural heritage platform.",
			Long: `A command line for the Europeana cultural heritage platform.

europeana searches and retrieves cultural heritage items from api.europeana.eu —
paintings, manuscripts, photos, books, and more from 66 million objects across
European museums and libraries. Uses the public demo API key by default;
set EUROPEANA_KEY to use your own.`,
			Site: Host,
			Repo: "https://github.com/tamnd/europeana-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "search", Group: "read", List: true,
		Summary: "Search Europeana cultural heritage items",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}}}, searchItems)

	kit.Handle(app, kit.OpMeta{Name: "record", Group: "read", Single: true,
		Summary: "Get a single record by Europeana ID",
		Args:    []kit.Arg{{Name: "id", Help: "Europeana record ID (e.g. /91619/SMVK_EM_objekt_1059045)"}}}, getRecord)
}

// newClient builds the Client from kit config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	// Allow API key override from kit config Extra map or environment variable
	if key := cfg.Extra["apikey"]; key != "" {
		c.APIKey = key
	} else if key := os.Getenv("EUROPEANA_KEY"); key != "" {
		c.APIKey = key
	}
	return NewClient(c), nil
}

// --- input structs ---

type searchInput struct {
	Query  string  `kit:"arg" help:"search query"`
	Type   string  `kit:"flag" help:"item type: IMAGE|TEXT|VIDEO|SOUND"`
	Start  int     `kit:"flag" help:"pagination start (1-based)"`
	Rows   int     `kit:"flag,inherit" help:"results per page (default 25)"`
	Client *Client `kit:"inject"`
}

type recordInput struct {
	ID     string  `kit:"arg" help:"Europeana record ID"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func searchItems(ctx context.Context, in searchInput, emit func(*Item) error) error {
	items, err := in.Client.Search(ctx, in.Query, in.Type, in.Start, in.Rows)
	if err != nil {
		return err
	}
	for i := range items {
		if err := emit(&items[i]); err != nil {
			return err
		}
	}
	return nil
}

func getRecord(ctx context.Context, in recordInput, emit func(*Item) error) error {
	item, err := in.Client.GetRecord(ctx, in.ID)
	if err != nil {
		return err
	}
	return emit(item)
}

// Classify turns any input into (type, id).
func (Domain) Classify(input string) (string, string, error) {
	return "item", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(t, id string) (string, error) {
	switch t {
	case "item":
		return "https://www.europeana.eu/item" + id, nil
	default:
		return "", errs.Usage("europeana has no resource type %q", t)
	}
}

