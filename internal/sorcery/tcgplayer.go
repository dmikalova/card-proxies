package sorcery

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	// Registers JPEG for image.Decode, for TCGplayer images.
	_ "image/jpeg"
	"image/png"
	"net/http"
	"slices"
	"strings"
	"unicode"
)

// TCGplayer serves sharper renders of the Alpha and Beta cards than the
// official image CDN: 750 x 1048 against 380 x 531.
const (
	tcgcsvProductsFormat = "https://tcgcsv.com/tcgplayer/77/%d/products"
	tcgplayerImageFormat = "https://product-images.tcgplayer.com/%d.jpg"
)

// tcgplayerGroups maps the set codes with low resolution official images to
// their TCGplayer group IDs.
var tcgplayerGroups = map[string]int{
	"001": 23335, // Alpha
	"002": 23336, // Beta
}

// TCGplayerProduct maps one printing to its TCGplayer product.
type TCGplayerProduct struct {
	Printing  string `json:"printing"`
	Name      string `json:"name"`
	ProductID int    `json:"product_id"`
}

// ImageURL is the address of the product's full size image.
func (p TCGplayerProduct) ImageURL() string {
	return fmt.Sprintf(tcgplayerImageFormat, p.ProductID)
}

type tcgcsvProducts struct {
	Results []struct {
		Name      string `json:"name"`
		ProductID int    `json:"productId"`
	} `json:"results"`
}

// FetchTCGplayerProducts reads the TCGplayer product lists of the sets in
// tcgplayerGroups and maps each Standard Booster printing to the product
// with the card's name and no suffix such as "(Foil)". It returns the map
// sorted by printing, and the printings without a product.
func FetchTCGplayerProducts(
	ctx context.Context,
	client *http.Client,
	catalog *Catalog,
) ([]TCGplayerProduct, []string, error) {
	var products []TCGplayerProduct
	var unmatched []string
	for set, group := range tcgplayerGroups {
		// tcgcsv.com refuses the default Go User-Agent.
		header := http.Header{"User-Agent": {"card-proxies"}}
		body, err := get(ctx, client, fmt.Sprintf(tcgcsvProductsFormat, group), header)
		if err != nil {
			return nil, nil, err
		}
		var list tcgcsvProducts
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, nil, fmt.Errorf("parse the TCGplayer products of set %s: %w", set, err)
		}
		byName := make(map[string]int, len(list.Results))
		for _, r := range list.Results {
			byName[matchKey(r.Name)] = r.ProductID
		}
		for i := range catalog.Cards {
			card := &catalog.Cards[i]
			for _, p := range card.Printings {
				if p.Set.Code != set || p.Meta.Finish != "Standard" || p.Meta.Product != "Booster" {
					continue
				}
				id, ok := byName[matchKey(card.Name)]
				if !ok {
					unmatched = append(unmatched, p.Slug)
					continue
				}
				products = append(
					products,
					TCGplayerProduct{Printing: p.Slug, Name: card.Name, ProductID: id},
				)
			}
		}
	}
	slices.SortFunc(
		products,
		func(a, b TCGplayerProduct) int { return cmp.Compare(a.Printing, b.Printing) },
	)
	slices.Sort(unmatched)
	return products, unmatched, nil
}

// matchKey folds a card name to lower case letters and digits, so that
// punctuation and accents do not stop a match.
func matchKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r == 'æ':
			b.WriteString("ae")
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsLetter(r):
			b.WriteRune(foldAccent(r))
		}
	}
	return b.String()
}

func foldAccent(r rune) rune {
	for base, accented := range map[rune]string{
		'a': "àáâãäå", 'e': "èéêë", 'i': "ìíîï", 'o': "òóôõöø", 'u': "ùúûü", 'n': "ñ", 'c': "ç", 'y': "ýÿ",
	} {
		if strings.ContainsRune(accented, r) {
			return base
		}
	}
	return r
}

// MarshalTCGplayerProducts writes the map as indented JSON.
func MarshalTCGplayerProducts(products []TCGplayerProduct) ([]byte, error) {
	data, err := json.MarshalIndent(products, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the TCGplayer products: %w", err)
	}
	return append(data, '\n'), nil
}

// ParseTCGplayerProducts reads tcgplayer.json into a map by printing slug.
func ParseTCGplayerProducts(data []byte) (map[string]TCGplayerProduct, error) {
	var products []TCGplayerProduct
	if err := json.Unmarshal(data, &products); err != nil {
		return nil, fmt.Errorf("parse the TCGplayer products: %w", err)
	}
	bySlug := make(map[string]TCGplayerProduct, len(products))
	for _, p := range products {
		bySlug[p.Printing] = p
	}
	return bySlug, nil
}

// CornerMask is the shape of a card: the alpha channel of an official card
// image, whose corners are transparent.
type CornerMask struct {
	alpha *image.Alpha
}

// NewCornerMask takes the alpha channel of a PNG card image as the mask.
func NewCornerMask(templatePNG []byte) (*CornerMask, error) {
	img, err := png.Decode(bytes.NewReader(templatePNG))
	if err != nil {
		return nil, fmt.Errorf("decode the corner template: %w", err)
	}
	b := img.Bounds()
	alpha := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(alpha, alpha.Bounds(), img, b.Min, draw.Src)
	return &CornerMask{alpha: alpha}, nil
}

// Round decodes a card image, makes its corners transparent with the mask
// scaled to the image size, and encodes it as PNG.
func (m *CornerMask) Round(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode the card image: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	mb := m.alpha.Bounds()
	for y := range h {
		for x := range w {
			a := m.sample(
				float64(x)*float64(mb.Dx())/float64(w),
				float64(y)*float64(mb.Dy())/float64(h),
			)
			c := out.NRGBAAt(x, y)
			c.A = uint8(uint16(c.A) * uint16(a) / 255)
			out.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, fmt.Errorf("encode the card image: %w", err)
	}
	return buf.Bytes(), nil
}

// sample reads the mask at a fractional position with bilinear filtering.
func (m *CornerMask) sample(fx, fy float64) uint8 {
	b := m.alpha.Bounds()
	at := func(x, y int) float64 {
		x = min(max(x, 0), b.Dx()-1)
		y = min(max(y, 0), b.Dy()-1)
		return float64(m.alpha.AlphaAt(x, y).A)
	}
	x0, y0 := int(fx), int(fy)
	tx, ty := fx-float64(x0), fy-float64(y0)
	top := at(x0, y0)*(1-tx) + at(x0+1, y0)*tx
	bottom := at(x0, y0+1)*(1-tx) + at(x0+1, y0+1)*tx
	return uint8(top*(1-ty) + bottom*ty + 0.5)
}
