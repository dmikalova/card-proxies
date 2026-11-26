package sorcery

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
)

// Paths are where the files of the game live.
type Paths struct {
	// Data holds the committed files: the catalog, the wants, the owned
	// cards and the ledger.
	Data string
	// Backs holds the card back images, which are not committed.
	Backs string
	// Cache holds the downloaded card images.
	Cache string
	// Runs holds one directory for each staged run.
	Runs string
}

// NewPaths places the game's files under the project root.
func NewPaths(root string) Paths {
	return Paths{
		Data:  filepath.Join(root, "data", "sorcery"),
		Backs: filepath.Join(root, "backs", "sorcery"),
		Cache: filepath.Join(root, "cache", "sorcery"),
		Runs:  filepath.Join(root, "runs", "sorcery"),
	}
}

// Catalog is the committed copy of the card catalog.
func (p Paths) Catalog() string { return filepath.Join(p.Data, "cards.json") }

// Config is the hand-edited PDF settings and want groups.
func (p Paths) Config() string { return filepath.Join(p.Data, "proxies.toml") }

// Owned is the hand-edited owned groups.
func (p Paths) Owned() string { return filepath.Join(p.Data, "owned.toml") }

// Ledger is the record of printed runs.
func (p Paths) Ledger() string { return filepath.Join(p.Data, "printed.json") }

// Run is the directory of one staged run.
func (p Paths) Run(id string) string { return filepath.Join(p.Runs, id) }

// UpdateCatalog downloads the catalog into its committed copy.
func UpdateCatalog(ctx context.Context, client *http.Client, p Paths) error {
	data, err := FetchCatalog(ctx, client, CatalogURL)
	if err != nil {
		return err
	}
	return writeFileAtomic(p.Catalog(), data)
}

// Planned is a plan with the config it came from.
type Planned struct {
	Config  *Config
	Entries []Entry
	// OwnedFrom names the file of the owned groups, or is empty if there is
	// none.
	OwnedFrom string
}

// Plan builds the plan of the next run from the committed files. It changes
// nothing.
func Plan(p Paths) (*Planned, error) {
	catalog, err := loadCatalog(p)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	lines, err := Resolve(catalog, cfg.Wants)
	if err != nil {
		return nil, err
	}
	owned, ownedFrom, err := loadOwned(p, catalog)
	if err != nil {
		return nil, err
	}
	ledger, err := loadLedger(p)
	if err != nil {
		return nil, err
	}
	return &Planned{
		Config:    cfg,
		Entries:   BuildPlan(lines, owned, ledger.PrintedByCard()),
		OwnedFrom: ownedFrom,
	}, nil
}

// LoadConfig reads proxies.toml.
func LoadConfig(p Paths) (*Config, error) {
	data, err := os.ReadFile(p.Config())
	if err != nil {
		return nil, fmt.Errorf("read the config: %w", err)
	}
	return ParseConfig(data)
}

func loadCatalog(p Paths) (*Catalog, error) {
	data, err := os.ReadFile(p.Catalog())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist: run mage sorcery:catalog", p.Catalog())
	}
	if err != nil {
		return nil, fmt.Errorf("read the catalog: %w", err)
	}
	return ParseCatalog(data)
}

// loadOwned reads owned.toml. Without it, nothing is owned.
func loadOwned(p Paths, catalog *Catalog) (map[string]int, string, error) {
	data, err := readOptional(p.Owned())
	if err != nil {
		return nil, "", err
	}
	if data == nil {
		return map[string]int{}, "", nil
	}
	owned, err := ParseOwned(catalog, data)
	if err != nil {
		return nil, "", err
	}
	return owned, p.Owned(), nil
}

func loadLedger(p Paths) (*Ledger, error) {
	data, err := readOptional(p.Ledger())
	if err != nil {
		return nil, err
	}
	return ParseLedger(data)
}

// Staged is a staged run and what its report needs.
type Staged struct {
	*Planned
	Dir    string
	Images map[string]ImageInfo
	// Backs reports whether the run has card backs. Without them the PDF
	// prints fronts only.
	Backs bool
}

// StageRun plans the next run, downloads its card images and writes it to
// the run directory of id.
func StageRun(ctx context.Context, client *http.Client, p Paths, id string) (*Staged, error) {
	planned, err := Plan(p)
	if err != nil {
		return nil, err
	}
	backs, err := LoadBacks(p.Backs)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]string)
	infos := make(map[string]ImageInfo)
	for i := range planned.Entries {
		e := &planned.Entries[i]
		if e.Print == 0 {
			continue
		}
		path, info, err := CachedImage(ctx, client, p.Cache, e)
		if err != nil {
			return nil, err
		}
		paths[e.Printing] = path
		infos[e.Printing] = info
	}
	dir := p.Run(id)
	if err := Stage(dir, planned.Entries, paths, backs); err != nil {
		return nil, err
	}
	return &Staged{Planned: planned, Dir: dir, Images: infos, Backs: backs != nil}, nil
}

// LatestRun is the ID of the newest staged run.
func LatestRun(p Paths) (string, error) {
	entries, err := os.ReadDir(p.Runs)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read %s: %w", p.Runs, err)
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	if len(ids) == 0 {
		return "", errors.New("no staged run: run mage sorcery:stage")
	}
	return slices.Max(ids), nil
}

// RecordRun adds a staged run to the ledger.
func RecordRun(p Paths, id string) error {
	data, err := os.ReadFile(filepath.Join(p.Run(id), planName))
	if err != nil {
		return fmt.Errorf("read the plan of run %s: %w", id, err)
	}
	entries, err := ParsePlan(data)
	if err != nil {
		return err
	}
	ledger, err := loadLedger(p)
	if err != nil {
		return err
	}
	if err := ledger.Record(id, entries); err != nil {
		return err
	}
	out, err := ledger.Marshal()
	if err != nil {
		return err
	}
	return writeFileAtomic(p.Ledger(), out)
}

// PDFArgs are the create_pdf.py arguments for a staged run: the run's
// directories and output path, then the config's flags. A run without card
// backs prints fronts only.
func PDFArgs(p Paths, id string, cfg *Config) ([]string, error) {
	dir := p.Run(id)
	_, err := os.Stat(filepath.Join(dir, backDir, spellbookBackName))
	withBacks := err == nil
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("check the backs of run %s: %w", id, err)
	}
	args := []string{
		"--front_dir_path", filepath.Join(dir, frontDir),
		"--back_dir_path", filepath.Join(dir, backDir),
		"--double_sided_dir_path", filepath.Join(dir, doubleSidedDir),
		"--output_path", filepath.Join(dir, "sorcery.pdf"),
	}
	if !withBacks {
		args = append(args, "--only_fronts")
	}
	return append(args, cfg.PDF.Args...), nil
}
