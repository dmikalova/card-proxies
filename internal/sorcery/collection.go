package sorcery

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"go.yaml.in/yaml/v3"
)

// QuantitiesURL is the tRPC procedure that sorcerytcg.com's own pages use to
// read the owned quantities. It is not a documented API, so it can change
// without notice. It needs the session cookie of a signed-in browser.
const QuantitiesURL = "https://sorcerytcg.com/api/trpc/inventory.getQuantities"

// quantitiesBatchSize is the number of card IDs in one request. The
// sorcerytcg.com browser extension that this follows sends 100.
const quantitiesBatchSize = 100

// OwnedPrinting is the owned quantity of one printing.
type OwnedPrinting struct {
	PrintingID string `json:"printing_id"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Quantity   int    `json:"quantity"`
}

// ErrNotSignedIn means that sorcerytcg.com refused the session cookie.
var ErrNotSignedIn = errors.New(
	"sorcerytcg.com refused the session cookie: copy a new cookie from a signed-in browser",
)

// FetchCollection reads the owned quantity of every printing in the catalog
// from sorcerytcg.com, with the browser's session cookie. It returns the
// printings with a quantity above zero, sorted by slug.
func FetchCollection(
	ctx context.Context,
	client *http.Client,
	endpoint, cookie string,
	catalog *Catalog,
) ([]OwnedPrinting, error) {
	ids := make([]string, 0, len(catalog.Cards))
	for i := range catalog.Cards {
		ids = append(ids, catalog.Cards[i].ID)
	}
	var owned []OwnedPrinting
	for batch := range slices.Chunk(ids, quantitiesBatchSize) {
		quantities, err := fetchQuantities(ctx, client, endpoint, cookie, batch)
		if err != nil {
			return nil, err
		}
		for printingID, q := range quantities {
			if q.Quantity <= 0 {
				continue
			}
			op := OwnedPrinting{PrintingID: printingID, Quantity: q.Quantity}
			if card, ok := catalog.CardOfPrinting(printingID); ok {
				op.Name = card.Name
				op.Slug = slugOf(card, printingID)
			}
			owned = append(owned, op)
		}
	}
	slices.SortFunc(owned, func(a, b OwnedPrinting) int {
		return cmp.Or(cmp.Compare(a.Slug, b.Slug), cmp.Compare(a.PrintingID, b.PrintingID))
	})
	return owned, nil
}

// ownedQuantity is the value of each printing ID in the tRPC response.
type ownedQuantity struct {
	Quantity int `json:"quantity"`
}

// trpcResponse is one entry of a tRPC batch response.
type trpcResponse struct {
	Result *struct {
		Data struct {
			JSON map[string]ownedQuantity `json:"json"`
		} `json:"data"`
	} `json:"result"`
	Error *struct {
		JSON struct {
			Message string `json:"message"`
		} `json:"json"`
	} `json:"error"`
}

func fetchQuantities(
	ctx context.Context,
	client *http.Client,
	endpoint, cookie string,
	cardIDs []string,
) (map[string]ownedQuantity, error) {
	input, err := json.Marshal(
		map[string]any{"0": map[string]any{"json": map[string]any{"cardIds": cardIDs}}},
	)
	if err != nil {
		return nil, fmt.Errorf("encode the request: %w", err)
	}
	query := url.Values{"batch": {"1"}, "input": {string(input)}}
	header := http.Header{"Cookie": {cookie}, "Accept": {"application/json"}}
	body, err := get(ctx, client, endpoint+"?"+query.Encode(), header)
	var httpErr *HTTPError
	if errors.As(err, &httpErr) &&
		(httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden) {
		return nil, ErrNotSignedIn
	}
	if err != nil {
		return nil, err
	}
	var responses []trpcResponse
	if err := json.Unmarshal(body, &responses); err != nil {
		return nil, fmt.Errorf("parse the owned quantities: %w", err)
	}
	if len(responses) == 0 {
		return nil, errors.New("parse the owned quantities: empty response")
	}
	r := responses[0]
	if r.Error != nil {
		return nil, fmt.Errorf("sorcerytcg.com: %s", r.Error.JSON.Message)
	}
	if r.Result == nil {
		return nil, errors.New("parse the owned quantities: no result")
	}
	return r.Result.Data.JSON, nil
}

func slugOf(card *Card, printingID string) string {
	for _, p := range card.Printings {
		if p.ID == printingID {
			return p.Slug
		}
	}
	return ""
}

// MarshalCollection writes the owned printings as indented JSON.
func MarshalCollection(owned []OwnedPrinting) ([]byte, error) {
	data, err := json.MarshalIndent(owned, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the collection: %w", err)
	}
	return append(data, '\n'), nil
}

// OwnedByCard adds up the owned printings of each card, keyed by card ID.
func OwnedByCard(catalog *Catalog, owned []OwnedPrinting) map[string]int {
	byCard := make(map[string]int)
	for _, op := range owned {
		if card, ok := catalog.CardOfPrinting(op.PrintingID); ok {
			byCard[card.ID] += op.Quantity
		}
	}
	return byCard
}

// ParseCollection reads collection.json.
func ParseCollection(data []byte) ([]OwnedPrinting, error) {
	var owned []OwnedPrinting
	if err := json.Unmarshal(data, &owned); err != nil {
		return nil, fmt.Errorf("parse the collection: %w", err)
	}
	return owned, nil
}

// ownedFile is owned.yaml: owned quantities by card name, for use without
// the sorcerytcg.com collection.
type ownedFile struct {
	Owned map[string]int `yaml:"owned"`
}

// ParseOwned reads owned.yaml and returns the owned quantities by card ID.
// It reports every name that the catalog does not have.
func ParseOwned(catalog *Catalog, data []byte) (map[string]int, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f ownedFile
	// A file of only comments decodes to io.EOF, and means that nothing is
	// owned.
	if err := dec.Decode(&f); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("parse owned.yaml: %w", err)
	}
	byCard := make(map[string]int, len(f.Owned))
	var missing []string
	for name, q := range f.Owned {
		card, ok := catalog.Lookup(name)
		if !ok {
			missing = append(missing, name)
			continue
		}
		byCard[card.ID] += q
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("owned.yaml: cards not in the catalog: %v", missing)
	}
	return byCard, nil
}
