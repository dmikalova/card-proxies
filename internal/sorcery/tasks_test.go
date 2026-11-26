package sorcery

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const testConfig = `pdf:
  args: [--paper_size, tabloid]
wants:
  - group: uniques
    sets: ["002", "004"]
    rarity: Unique
`

func testCatalogJSON() string {
	return `[
  {"id": "galahad", "name": "Sir Galahad", "slug": "sir_galahad",
   "engine": {"type": "Minion", "rarity": "Unique", "back": null},
   "printings": [{"id": "galahad-s", "slug": "004-sir_galahad-b-s", "printedAt": "2024-10-04T07:00:00.000Z",
     "set": {"name": "Arthurian Legends", "code": "004"}, "meta": {"finish": "Standard", "product": "Booster"}}]},
  {"id": "vesuvius", "name": "Vesuvius", "slug": "vesuvius",
   "engine": {"type": "Site", "rarity": "Unique", "back": null},
   "printings": [{"id": "vesuvius-beta", "slug": "002-vesuvius-b-s", "printedAt": "2023-01-01T00:00:00.000Z",
     "set": {"name": "Beta", "code": "002"}, "meta": {"finish": "Standard", "product": "Booster"}}]}
]`
}

func TestFetchCatalog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Join(strings.Fields(testCatalogJSON()), " "))
	}))
	defer srv.Close()

	data, err := FetchCatalog(t.Context(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("FetchCatalog: %v", err)
	}
	if !bytes.Contains(data, []byte("\n  {\n")) {
		t.Error("FetchCatalog did not indent the catalog")
	}
	c, err := ParseCatalog(data)
	if err != nil {
		t.Fatalf("ParseCatalog: %v", err)
	}
	if card, ok := c.CardOfPrinting("vesuvius-beta"); !ok || card.Name != "Vesuvius" {
		t.Errorf("CardOfPrinting = %v, %v", card, ok)
	}
}

func TestFetchCatalogRejectsBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"error": "down"}`)
	}))
	defer srv.Close()

	if _, err := FetchCatalog(t.Context(), srv.Client(), srv.URL); err == nil {
		t.Error("FetchCatalog accepted a response that is not a card list")
	}
}

// testProject writes a project with the test catalog and config, and an
// image server for its cards.
func testProject(t *testing.T) (Paths, *httptest.Server) {
	t.Helper()
	p := NewPaths(t.TempDir())
	writeFile(t, p.Catalog(), []byte(testCatalogJSON()))
	writeFile(t, p.Config(), []byte(testConfig))
	img := pngBytes(t, 744, 1039)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(img)
	}))
	t.Cleanup(srv.Close)
	return p, srv
}

// redirectImages points the plan's image URLs at the test server.
func redirectImages(srv *httptest.Server) *http.Client {
	client := srv.Client()
	client.Transport = rewriteTransport{target: srv.URL, base: client.Transport}
	return client
}

type rewriteTransport struct {
	target string
	base   http.RoundTripper
}

func (rt rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, rt.target+r.URL.Path, http.NoBody)
	if err != nil {
		return nil, err
	}
	return rt.base.RoundTrip(req)
}

func TestStageAndRecordRun(t *testing.T) {
	p, srv := testProject(t)
	writeFile(t, p.Owned(), []byte("owned:\n  Sir Galahad: 1\n"))

	staged, err := StageRun(t.Context(), redirectImages(srv), p, "2026-11-25-1200")
	if err != nil {
		t.Fatalf("StageRun: %v", err)
	}
	if staged.OwnedFrom != p.Owned() || staged.Backs {
		t.Errorf("StageRun = owned from %q, backs %v", staged.OwnedFrom, staged.Backs)
	}
	if got := listDir(
		t,
		filepath.Join(staged.Dir, frontDir),
	); !slices.Equal(
		got,
		[]string{"001-002-vesuvius-b-s-1.png"},
	) {
		t.Errorf("front = %v", got)
	}

	id, err := LatestRun(p)
	if err != nil || id != "2026-11-25-1200" {
		t.Fatalf("LatestRun = %q, %v", id, err)
	}
	args, err := PDFArgs(p, id, staged.Config)
	if err != nil {
		t.Fatalf("PDFArgs: %v", err)
	}
	if !slices.Contains(args, "--only_fronts") || args[len(args)-1] != "tabloid" {
		t.Errorf("PDFArgs = %v, want --only_fronts and the config flags last", args)
	}

	if err := RecordRun(p, id); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if err := RecordRun(p, id); err == nil {
		t.Error("RecordRun recorded the same run twice")
	}
	planned, err := Plan(p)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if total := Total(planned.Entries); total.Titles != 0 {
		t.Errorf("Plan after the record prints %+v, want nothing", total)
	}
}

func TestPDFArgsWithBacks(t *testing.T) {
	p := NewPaths(t.TempDir())
	writeFile(t, filepath.Join(p.Run("r1"), backDir, spellbookBackName), []byte("png"))

	args, err := PDFArgs(p, "r1", &Config{})
	if err != nil {
		t.Fatalf("PDFArgs: %v", err)
	}
	if slices.Contains(args, "--only_fronts") {
		t.Errorf("PDFArgs = %v, want backs", args)
	}
}

func TestLatestRunWithoutRuns(t *testing.T) {
	if _, err := LatestRun(NewPaths(t.TempDir())); err == nil {
		t.Error("LatestRun found a run in an empty project")
	}
}

func TestPlanWithoutCatalog(t *testing.T) {
	p := NewPaths(t.TempDir())
	if _, err := Plan(p); err == nil || !strings.Contains(err.Error(), "mage sorcery:catalog") {
		t.Errorf("Plan error = %v, want a hint to run mage sorcery:catalog", err)
	}
}

func TestPlanPrefersCollection(t *testing.T) {
	p, _ := testProject(t)
	writeFile(t, p.Owned(), []byte("owned:\n  Not A Card: 1\n"))
	writeFile(t, p.Collection(), []byte(`[{"printing_id": "vesuvius-beta", "quantity": 1}]`))

	planned, err := Plan(p)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if planned.OwnedFrom != p.Collection() {
		t.Errorf("OwnedFrom = %s, want the collection", planned.OwnedFrom)
	}
	if total := Total(planned.Entries); total.Titles != 1 || total.Atlas != 0 {
		t.Errorf("Total = %+v, want only Sir Galahad", total)
	}
}

func TestReport(t *testing.T) {
	entries := []Entry{
		{
			Name:        "Druid",
			Type:        "Avatar",
			Printing:    "004-druid-bt-s",
			Groups:      []string{"avatars"},
			Want:        1,
			Print:       1,
			DoubleFaced: true,
		},
		{
			Name:     "Fireball",
			Type:     "Magic",
			Printing: "002-fireball-b-s",
			Groups:   []string{"errata"},
			Want:     3,
			Print:    3,
		},
		{Name: "Done", Type: "Magic", Printing: "x", Want: 1, Owned: 1},
	}
	r := Report{
		Entries: entries,
		Images:  map[string]ImageInfo{"002-fireball-b-s": {Width: 380, Height: 531}},
		Staged:  true,
	}
	var b strings.Builder
	if err := r.Write(&b); err != nil {
		t.Fatalf("Write: %v", err)
	}
	out := b.String()
	for _, want := range []string{
		"Owned: nothing",
		"Cards to print: 2. Spellbook copies: 4. Atlas copies: 0.",
		"Cards already owned or printed: 1.",
		"Fireball (153 PPI)",
		"The run prints the front face only:\n  Druid",
		"the PDF prints fronts only",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not contain %q:\n%s", want, out)
		}
	}
}
