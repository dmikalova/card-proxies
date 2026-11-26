package sorcery

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// quantitiesServer answers inventory.getQuantities from owned, keyed by
// printing ID, for the card IDs of each request.
func quantitiesServer(
	t *testing.T,
	c *Catalog,
	owned map[string]int,
	requests *int,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++
		if r.Header.Get("Cookie") != "session=abc" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var input map[string]struct {
			JSON struct {
				CardIDs []string `json:"cardIds"`
			} `json:"json"`
		}
		if err := json.Unmarshal([]byte(r.URL.Query().Get("input")), &input); err != nil {
			t.Errorf("input: %v", err)
		}
		result := map[string]map[string]int{}
		for i := range c.Cards {
			card := &c.Cards[i]
			if !strings.Contains(strings.Join(input["0"].JSON.CardIDs, ","), card.ID) {
				continue
			}
			for _, p := range card.Printings {
				result[p.ID] = map[string]int{"quantity": owned[p.ID]}
			}
		}
		_ = json.NewEncoder(w).
			Encode([]any{map[string]any{"result": map[string]any{"data": map[string]any{"json": result}}}})
	}))
}

func TestFetchCollection(t *testing.T) {
	c := testCatalog(t)
	requests := 0
	srv := quantitiesServer(t, c, map[string]int{"galahad-f": 1, "vesuvius-beta": 2}, &requests)
	defer srv.Close()

	owned, err := FetchCollection(t.Context(), srv.Client(), srv.URL, "session=abc", c)
	if err != nil {
		t.Fatalf("FetchCollection: %v", err)
	}
	want := []OwnedPrinting{
		{PrintingID: "vesuvius-beta", Slug: "002-vesuvius-b-s", Name: "Vesuvius", Quantity: 2},
		{PrintingID: "galahad-f", Slug: "004-sir_galahad-b-f", Name: "Sir Galahad", Quantity: 1},
	}
	if len(owned) != len(want) || owned[0] != want[0] || owned[1] != want[1] {
		t.Errorf("FetchCollection =\n%+v\nwant\n%+v", owned, want)
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1 for %d cards", requests, len(c.Cards))
	}
	if got := OwnedByCard(c, owned); got["galahad"] != 1 || got["vesuvius"] != 2 {
		t.Errorf("OwnedByCard = %v", got)
	}

	data, err := MarshalCollection(owned)
	if err != nil {
		t.Fatalf("MarshalCollection: %v", err)
	}
	back, err := ParseCollection(data)
	if err != nil || len(back) != 2 {
		t.Errorf("ParseCollection = %v, %v", back, err)
	}
}

func TestFetchCollectionNotSignedIn(t *testing.T) {
	c := testCatalog(t)
	requests := 0
	srv := quantitiesServer(t, c, nil, &requests)
	defer srv.Close()

	_, err := FetchCollection(t.Context(), srv.Client(), srv.URL, "session=old", c)
	if !errors.Is(err, ErrNotSignedIn) {
		t.Errorf("FetchCollection error = %v, want ErrNotSignedIn", err)
	}
}

func TestFetchCollectionTRPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `[{"error":{"json":{"message":"Too many card IDs"}}}]`)
	}))
	defer srv.Close()

	_, err := FetchCollection(t.Context(), srv.Client(), srv.URL, "session=abc", testCatalog(t))
	if err == nil || !strings.Contains(err.Error(), "Too many card IDs") {
		t.Errorf("FetchCollection error = %v", err)
	}
}

func TestParseOwned(t *testing.T) {
	c := testCatalog(t)
	owned, err := ParseOwned(c, []byte("owned:\n  sir galahad: 1\n  Vesuvius: 2\n"))
	if err != nil {
		t.Fatalf("ParseOwned: %v", err)
	}
	if owned["galahad"] != 1 || owned["vesuvius"] != 2 {
		t.Errorf("ParseOwned = %v", owned)
	}

	owned, err = ParseOwned(c, []byte("# Only comments.\n"))
	if err != nil || len(owned) != 0 {
		t.Errorf("ParseOwned of comments = %v, %v", owned, err)
	}

	if _, err := ParseOwned(c, []byte("owned:\n  Sir Galahadd: 1\n")); err == nil {
		t.Error("ParseOwned accepted a name not in the catalog")
	}
}
