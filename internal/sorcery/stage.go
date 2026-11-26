package sorcery

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
)

// cardWidthInches is the width of a standard card, 63 mm.
const cardWidthInches = 63.0 / 25.4

// MinPPI is the resolution below which a card image prints soft.
const MinPPI = 250

// Back image names in the backs directory.
const (
	spellbookBackName = "spellbook.png"
	atlasBackName     = "atlas.png"
)

// Run directories that silhouette-card-maker reads.
const (
	frontDir       = "front"
	backDir        = "back"
	doubleSidedDir = "double_sided"
	planName       = "plan.json"
)

// Backs are the PNG card backs of a run.
type Backs struct {
	Spellbook []byte
	// Atlas is in portrait orientation, as the card images show Sites.
	Atlas []byte
}

// LoadBacks reads spellbook.png and atlas.png from dir. It returns nil if
// neither exists, which prints fronts only. One without the other is an
// error, because Sites would get the spellbook back.
func LoadBacks(dir string) (*Backs, error) {
	spellbook, err := readOptional(filepath.Join(dir, spellbookBackName))
	if err != nil {
		return nil, err
	}
	atlas, err := readOptional(filepath.Join(dir, atlasBackName))
	if err != nil {
		return nil, err
	}
	switch {
	case spellbook == nil && atlas == nil:
		return nil, nil
	case spellbook == nil || atlas == nil:
		return nil, fmt.Errorf(
			"put both %s and %s in %s, or neither to print fronts only",
			spellbookBackName,
			atlasBackName,
			dir,
		)
	}
	portrait, err := portraitPNG(atlas)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", atlasBackName, err)
	}
	return &Backs{Spellbook: spellbook, Atlas: portrait}, nil
}

// readOptional reads a file, and returns nil data if it does not exist.
func readOptional(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// portraitPNG turns a landscape PNG a quarter turn clockwise. A portrait PNG
// stays as it is.
func portraitPNG(data []byte) ([]byte, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode the PNG: %w", err)
	}
	b := img.Bounds()
	if b.Dx() <= b.Dy() {
		return data, nil
	}
	var out bytes.Buffer
	if err := png.Encode(&out, rotateClockwise(img)); err != nil {
		return nil, fmt.Errorf("encode the PNG: %w", err)
	}
	return out.Bytes(), nil
}

func rotateClockwise(img image.Image) *image.NRGBA {
	b := img.Bounds()
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, h, w))
	for y := range h {
		for x := range w {
			dst.SetNRGBA(h-1-y, x, src.NRGBAAt(x, y))
		}
	}
	return dst
}

// ImageInfo is the size of a card image and its resolution at card size.
type ImageInfo struct {
	Width  int
	Height int
}

// PPI is the resolution of the image at the width of a card.
func (i ImageInfo) PPI() int {
	return int(float64(i.Width) / cardWidthInches)
}

// CachedImage downloads the image of the entry's printing into cacheDir,
// unless it is there already. It returns the image path and size.
func CachedImage(
	ctx context.Context,
	client *http.Client,
	cacheDir string,
	e *Entry,
) (string, ImageInfo, error) {
	path := filepath.Join(cacheDir, e.Printing+".png")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		data, err := get(ctx, client, e.ImageURL, nil)
		if err != nil {
			return "", ImageInfo{}, fmt.Errorf("image of %s: %w", e.Name, err)
		}
		if err := writeFileAtomic(path, data); err != nil {
			return "", ImageInfo{}, err
		}
	}
	info, err := readImageInfo(path)
	if err != nil {
		return "", ImageInfo{}, err
	}
	return path, info, nil
}

func readImageInfo(path string) (ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return ImageInfo{}, fmt.Errorf("read %s: %w", path, err)
	}
	return ImageInfo{Width: cfg.Width, Height: cfg.Height}, nil
}

// writeFileAtomic writes through a temporary file, so that an interrupted
// download leaves no partial image in the cache.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("make %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

// Stage writes a run to runDir: a front image for each copy, the spellbook
// back in back/, an atlas back in double_sided/ for each Site copy, and the
// plan. It empties those directories first, so that a run staged again holds
// no old files. images maps each printing slug to its cached image.
func Stage(runDir string, entries []Entry, images map[string]string, backs *Backs) error {
	for _, dir := range []string{frontDir, backDir, doubleSidedDir} {
		path := filepath.Join(runDir, dir)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("empty %s: %w", path, err)
		}
		if err := os.MkdirAll(path, 0o750); err != nil {
			return fmt.Errorf("make %s: %w", path, err)
		}
	}
	index := 0
	for i := range entries {
		e := &entries[i]
		if e.Print == 0 {
			continue
		}
		index++
		front, err := os.ReadFile(images[e.Printing])
		if err != nil {
			return fmt.Errorf("image of %s: %w", e.Name, err)
		}
		for n := 1; n <= e.Print; n++ {
			name := fmt.Sprintf("%03d-%s-%d.png", index, e.Printing, n)
			if err := os.WriteFile(
				filepath.Join(runDir, frontDir, name),
				front,
				0o600,
			); err != nil {
				return fmt.Errorf("write %s: %w", name, err)
			}
			if backs != nil && e.Atlas() {
				if err := os.WriteFile(
					filepath.Join(runDir, doubleSidedDir, name),
					backs.Atlas,
					0o600,
				); err != nil {
					return fmt.Errorf("write the atlas back of %s: %w", name, err)
				}
			}
		}
	}
	if backs != nil {
		if err := os.WriteFile(
			filepath.Join(runDir, backDir, spellbookBackName),
			backs.Spellbook,
			0o600,
		); err != nil {
			return fmt.Errorf("write the spellbook back: %w", err)
		}
	}
	plan, err := MarshalPlan(entries)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(runDir, planName), plan, 0o600); err != nil {
		return fmt.Errorf("write the plan: %w", err)
	}
	return nil
}
