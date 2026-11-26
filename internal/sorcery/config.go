package sorcery

// Config is the hand-edited proxies.toml of the game: the PDF settings and
// the want groups.
type Config struct {
	PDF    PDFConfig    `toml:"pdf"`
	Images ImagesConfig `toml:"images"`
	Wants  []Group      `toml:"want"`
}

// ImagesConfig holds the settings of the card images.
type ImagesConfig struct {
	// CornerTemplate is the printing slug of an official card image whose
	// transparent corners give the shape of the TCGplayer images.
	CornerTemplate string `toml:"corner_template"`
}

// PDFConfig holds the create_pdf.py flags of every run. The card directories
// and the output path are added for each run.
type PDFConfig struct {
	Args []string `toml:"args"`
}

// ParseConfig reads proxies.toml.
func ParseConfig(data []byte) (*Config, error) {
	var c Config
	if err := decodeTOML("proxies.toml", data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// ownedFile is owned.toml: the owned groups.
type ownedFile struct {
	Owned []Group `toml:"owned"`
}

// ParseOwned reads owned.toml and returns the owned copies of each printing,
// keyed by printing ID. An owned group with several sets and no named cards
// counts each set on its own, such as one playset of each set. Copies of a
// printing in two groups add up.
func ParseOwned(catalog *Catalog, data []byte) (map[string]int, error) {
	var f ownedFile
	if err := decodeTOML("owned.toml", data, &f); err != nil {
		return nil, err
	}
	var groups []Group
	for _, g := range f.Owned {
		if len(g.Sets) <= 1 || len(g.Cards) > 0 {
			groups = append(groups, g)
			continue
		}
		for _, set := range g.Sets {
			one := g
			one.Sets = []string{set}
			groups = append(groups, one)
		}
	}
	lines, err := Resolve(catalog, groups)
	if err != nil {
		return nil, err
	}
	owned := make(map[string]int)
	for _, l := range lines {
		owned[l.Printing.ID] += l.Quantity
	}
	return owned, nil
}
