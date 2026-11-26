// Package sorcery plans and stages proxy print runs for Sorcery: Contested
// Realm. It reads the public card catalog of sorcerytcg.com, the wants and the
// owned collection, and writes the card images that silhouette-card-maker
// turns into a PDF.
package sorcery

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

// CatalogURL is the public card catalog of sorcerytcg.com. It needs no login.
const CatalogURL = "https://api.sorcerytcg.com/api/cards"

// imageURLFormat is the card image address of a printing slug.
const imageURLFormat = "https://d27a44hjr9gen3.cloudfront.net/cards/%s.png"

// TypeSite is the card type of atlas cards. Every other type is a spellbook
// card.
const TypeSite = "Site"

// Card is one card of the catalog, with every printing of it.
type Card struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Slug      string     `json:"slug"`
	Engine    Engine     `json:"engine"`
	Printings []Printing `json:"printings"`
}

// Engine is the rules side of a card.
type Engine struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	// Rarity is empty for Avatars and tokens.
	Rarity string `json:"rarity"`
	// Back is the other face of a double-faced card, and nil for every other
	// card.
	Back *struct {
		Type string `json:"type"`
	} `json:"back"`
}

// Printing is one printing of a card: one set, finish and product.
type Printing struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	PrintedAt time.Time `json:"printedAt"`
	Set       Set       `json:"set"`
	Meta      Meta      `json:"meta"`
}

// Set is the set of a printing. Code is the number sorcerytcg.com uses, such
// as 002 for Beta.
type Set struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// Meta holds the finish and the product of a printing.
type Meta struct {
	Finish  string `json:"finish"`
	Product string `json:"product"`
}

// ImageURL is the address of the printing's card image.
func (p Printing) ImageURL() string {
	return fmt.Sprintf(imageURLFormat, p.Slug)
}

// Catalog is every card, with lookups by name and by printing.
type Catalog struct {
	Cards      []Card
	byName     map[string]*Card
	byPrinting map[string]*Card
}

// NewCatalog indexes the cards by name and by printing ID.
func NewCatalog(cards []Card) *Catalog {
	c := &Catalog{
		Cards:      cards,
		byName:     make(map[string]*Card, len(cards)),
		byPrinting: make(map[string]*Card, len(cards)),
	}
	for i := range c.Cards {
		card := &c.Cards[i]
		c.byName[normalizeName(card.Name)] = card
		for _, p := range card.Printings {
			c.byPrinting[p.ID] = card
		}
	}
	return c
}

// ParseCatalog reads the catalog JSON that CatalogURL serves.
func ParseCatalog(data []byte) (*Catalog, error) {
	var cards []Card
	if err := json.Unmarshal(data, &cards); err != nil {
		return nil, fmt.Errorf("parse the catalog: %w", err)
	}
	return NewCatalog(cards), nil
}

// Lookup finds a card by name. Case and outer spaces do not matter.
func (c *Catalog) Lookup(name string) (*Card, bool) {
	card, ok := c.byName[normalizeName(name)]
	return card, ok
}

// CardOfPrinting finds the card that a printing ID belongs to.
func (c *Catalog) CardOfPrinting(printingID string) (*Card, bool) {
	card, ok := c.byPrinting[printingID]
	return card, ok
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// FetchCatalog downloads the catalog from url and returns the committed form
// of it: the fields of Card only, sorted by slug and indented, so that a diff
// shows the new cards and printings. It leaves out the rules and flavor text,
// which the tool does not use. That text has typos, and the shared spell
// check would rewrite them in the committed copy.
func FetchCatalog(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	body, err := get(ctx, client, url, nil)
	if err != nil {
		return nil, err
	}
	catalog, err := ParseCatalog(body)
	if err != nil {
		return nil, err
	}
	cards := slices.Clone(catalog.Cards)
	slices.SortFunc(cards, func(a, b Card) int { return cmp.Compare(a.Slug, b.Slug) })
	for i := range cards {
		cards[i].Printings = slices.Clone(cards[i].Printings)
		slices.SortFunc(
			cards[i].Printings,
			func(a, b Printing) int { return cmp.Compare(a.Slug, b.Slug) },
		)
	}
	data, err := json.MarshalIndent(cards, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the catalog: %w", err)
	}
	return append(data, '\n'), nil
}

// get sends a GET request and returns the body of a 2XX response.
func get(ctx context.Context, client *http.Client, url string, header http.Header) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("make the request: %w", err)
	}
	for key, values := range header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", req.URL.Redacted(), err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", req.URL.Redacted(), err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &HTTPError{URL: req.URL.Redacted(), StatusCode: resp.StatusCode}
	}
	return body, nil
}

// HTTPError is a response with a status code outside 2XX.
type HTTPError struct {
	URL        string
	StatusCode int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("get %s: HTTP %d", e.URL, e.StatusCode)
}
