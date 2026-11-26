package sorcery

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b.Bytes()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestRotateClockwise(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	red := color.NRGBA{R: 255, A: 255}
	src.SetNRGBA(0, 0, red)

	dst := rotateClockwise(src)

	if dst.Bounds().Dx() != 2 || dst.Bounds().Dy() != 3 {
		t.Fatalf("size = %v, want 2x3", dst.Bounds())
	}
	// A quarter turn clockwise moves the top-left pixel to the top right.
	if dst.NRGBAAt(1, 0) != red {
		t.Error("the top-left pixel did not move to the top right")
	}
}

func TestLoadBacks(t *testing.T) {
	dir := t.TempDir()
	backs, err := LoadBacks(dir)
	if err != nil || backs != nil {
		t.Fatalf("LoadBacks with no backs = %v, %v; want nil, nil", backs, err)
	}

	writeFile(t, filepath.Join(dir, spellbookBack+".png"), pngBytes(t, 5, 7))
	if _, err := LoadBacks(dir); err == nil {
		t.Fatal("LoadBacks accepted a spellbook back without an atlas back")
	}

	writeFile(t, filepath.Join(dir, atlasBack+".jpeg"), jpegBytes(t, 7, 5))
	backs, err = LoadBacks(dir)
	if err != nil {
		t.Fatalf("LoadBacks: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(backs.Atlas))
	if err != nil {
		t.Fatalf("the atlas back is not a PNG: %v", err)
	}
	if cfg.Width != 5 || cfg.Height != 7 {
		t.Errorf(
			"atlas back = %dx%d, want the landscape JPEG turned to a 5x7 PNG",
			cfg.Width,
			cfg.Height,
		)
	}

	writeFile(t, filepath.Join(dir, atlasBack+".png"), pngBytes(t, 7, 5))
	if _, err := LoadBacks(
		dir,
	); err == nil ||
		!strings.Contains(err.Error(), "atlas.png and atlas.jpeg") {
		t.Errorf("LoadBacks error = %v, want a refusal of two atlas backs", err)
	}
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b.Bytes()
}

func TestStage(t *testing.T) {
	runDir := t.TempDir()
	cache := t.TempDir()
	writeFile(t, filepath.Join(runDir, frontDir, "old.png"), []byte("old"))
	writeFile(t, filepath.Join(cache, "fireball.png"), pngBytes(t, 5, 7))
	writeFile(t, filepath.Join(cache, "vesuvius.png"), pngBytes(t, 5, 7))
	entries := []Entry{
		{Name: "Done", Type: "Magic", Printing: "done", Print: 0},
		{Name: "Fireball", Type: "Magic", Printing: "fireball", Print: 2},
		{Name: "Vesuvius", Type: TypeSite, Printing: "vesuvius", Print: 1},
	}
	images := map[string]string{
		"fireball": filepath.Join(cache, "fireball.png"),
		"vesuvius": filepath.Join(cache, "vesuvius.png"),
	}
	backs := &Backs{Spellbook: pngBytes(t, 5, 7), Atlas: pngBytes(t, 5, 7)}

	if err := Stage(runDir, entries, images, backs); err != nil {
		t.Fatalf("Stage: %v", err)
	}

	if got, want := listDir(
		t,
		filepath.Join(runDir, frontDir),
	), []string{
		"001-fireball-1.png",
		"001-fireball-2.png",
		"002-vesuvius-1.png",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Errorf("front = %v, want %v", got, want)
	}
	if got, want := listDir(
		t,
		filepath.Join(runDir, doubleSidedDir),
	), []string{
		"002-vesuvius-1.png",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Errorf("double_sided = %v, want %v", got, want)
	}
	if got, want := listDir(
		t,
		filepath.Join(runDir, backDir),
	), []string{
		spellbookBackName,
	}; !slices.Equal(
		got,
		want,
	) {
		t.Errorf("back = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(runDir, planName)); err != nil {
		t.Errorf("plan: %v", err)
	}
}

func TestStageWithoutBacks(t *testing.T) {
	runDir := t.TempDir()
	cache := t.TempDir()
	writeFile(t, filepath.Join(cache, "vesuvius.png"), pngBytes(t, 5, 7))
	entries := []Entry{{Name: "Vesuvius", Type: TypeSite, Printing: "vesuvius", Print: 1}}

	if err := Stage(
		runDir,
		entries,
		map[string]string{"vesuvius": filepath.Join(cache, "vesuvius.png")},
		nil,
	); err != nil {
		t.Fatalf("Stage: %v", err)
	}
	if got := listDir(t, filepath.Join(runDir, doubleSidedDir)); len(got) != 0 {
		t.Errorf("double_sided = %v, want empty", got)
	}
}

func TestCachedImage(t *testing.T) {
	requests := 0
	img := pngBytes(t, 380, 531)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write(img)
	}))
	defer srv.Close()
	cache := t.TempDir()
	e := &Entry{Name: "Fireball", Printing: "002-fireball-b-s", ImageURL: srv.URL}

	for range 2 {
		path, info, err := CachedImage(t.Context(), srv.Client(), cache, e)
		if err != nil {
			t.Fatalf("CachedImage: %v", err)
		}
		if filepath.Base(path) != "002-fireball-b-s.png" || info.Width != 380 || info.PPI() != 153 {
			t.Errorf("CachedImage = %s, %+v (PPI %d)", path, info, info.PPI())
		}
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1: the second call reads the cache", requests)
	}
}

func TestCachedImageHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	e := &Entry{Name: "Missing", Printing: "missing", ImageURL: srv.URL}

	_, _, err := CachedImage(t.Context(), srv.Client(), t.TempDir(), e)
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("CachedImage error = %v, want HTTP 404", err)
	}
}
