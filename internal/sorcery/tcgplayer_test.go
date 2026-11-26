package sorcery

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestMatchKey(t *testing.T) {
	for in, want := range map[string]string{
		"Courtesan Thaïs":     "courtesanthais",
		"Philosopher's Stone": "philosophersstone",
		"Æther Bomb":          "aetherbomb",
		"Seven-League Boots":  "sevenleagueboots",
	} {
		if got := matchKey(in); got != want {
			t.Errorf("matchKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFetchTCGplayerProducts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "card-proxies" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		products := `{"results": []}`
		if strings.Contains(r.URL.Path, "/23336/") {
			products = `{"results": [
				{"name": "Fireball", "productId": 523373},
				{"name": "Fireball (Foil)", "productId": 523374},
				{"name": "Seirawan Hydra (Foil)", "productId": 1}
			]}`
		}
		_, _ = fmt.Fprint(w, products)
	}))
	defer srv.Close()

	products, unmatched, err := FetchTCGplayerProducts(
		t.Context(),
		redirectImages(srv),
		testCatalog(t),
	)
	if err != nil {
		t.Fatalf("FetchTCGplayerProducts: %v", err)
	}
	want := []TCGplayerProduct{{Printing: "002-fireball-b-s", Name: "Fireball", ProductID: 523373}}
	if !slices.Equal(products, want) {
		t.Errorf("products = %v, want %v", products, want)
	}
	if !slices.Contains(unmatched, "002-seirawan_hydra-b-s") ||
		slices.Contains(unmatched, "002-fireball-b-s") {
		t.Errorf("unmatched = %v, want Seirawan Hydra only among the matched Beta cards", unmatched)
	}
	if want[0].ImageURL() != "https://product-images.tcgplayer.com/523373.jpg" {
		t.Errorf("ImageURL = %s", want[0].ImageURL())
	}

	data, err := MarshalTCGplayerProducts(products)
	if err != nil {
		t.Fatalf("MarshalTCGplayerProducts: %v", err)
	}
	bySlug, err := ParseTCGplayerProducts(data)
	if err != nil || bySlug["002-fireball-b-s"].ProductID != 523373 {
		t.Errorf("ParseTCGplayerProducts = %v, %v", bySlug, err)
	}
}

// templatePNG is a 4 x 4 card shape with a transparent top-left pixel.
func templatePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.SetNRGBA(x, y, color.NRGBA{A: 255})
		}
	}
	img.SetNRGBA(0, 0, color.NRGBA{})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestCornerMaskRound(t *testing.T) {
	mask, err := NewCornerMask(templatePNG(t))
	if err != nil {
		t.Fatalf("NewCornerMask: %v", err)
	}
	out, err := mask.Round(jpegBytes(t, 8, 8))
	if err != nil {
		t.Fatalf("Round: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Round did not write a PNG: %v", err)
	}
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Errorf("corner alpha = %d, want 0", a)
	}
	if _, _, _, a := img.At(5, 5).RGBA(); a != 0xffff {
		t.Errorf("center alpha = %d, want opaque", a)
	}
}

func TestImageSourceUsesTCGplayer(t *testing.T) {
	p, srv := testProject(t)
	template := templatePNG(t)
	tcgplayer := jpegBytes(t, 8, 8)
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".jpg") {
			_, _ = w.Write(tcgplayer)
			return
		}
		_, _ = w.Write(template)
	})
	cfg := &Config{Images: ImagesConfig{CornerTemplate: "004-sir_galahad-b-s"}}
	writeFile(
		t,
		p.TCGplayer(),
		[]byte(`[{"printing": "002-vesuvius-b-s", "name": "Vesuvius", "product_id": 7}]`),
	)

	images, err := newImageSource(t.Context(), redirectImages(srv), p, cfg)
	if err != nil {
		t.Fatalf("newImageSource: %v", err)
	}
	e := &Entry{Name: "Vesuvius", Printing: "002-vesuvius-b-s"}
	path, info, err := images.fetch(t.Context(), e)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if filepath.Base(path) != "002-vesuvius-b-s-tcgplayer.png" || info.Width != 8 {
		t.Errorf("fetch = %s, %+v", path, info)
	}
	if e.ImageURL != "https://product-images.tcgplayer.com/7.jpg" {
		t.Errorf("ImageURL = %s, want the TCGplayer image", e.ImageURL)
	}

	if _, err := newImageSource(t.Context(), redirectImages(srv), p, &Config{}); err == nil {
		t.Error("newImageSource accepted TCGplayer products without a corner template")
	}
}
