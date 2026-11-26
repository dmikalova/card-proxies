package sorcery

import (
	"testing"
	"time"
)

var (
	older = time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	newer = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

func printing(id, slug, set, finish string, at time.Time) Printing {
	return Printing{
		ID:        id,
		Slug:      slug,
		PrintedAt: at,
		Set:       Set{Code: set},
		Meta:      Meta{Finish: finish, Product: "Booster"},
	}
}

// testCatalog is a small catalog with one card of each case the code
// handles.
func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	return NewCatalog([]Card{
		{
			ID: "galahad", Name: "Sir Galahad",
			Engine: Engine{Type: "Minion", Rarity: "Unique"},
			Printings: []Printing{
				printing("galahad-f", "004-sir_galahad-b-f", "004", "Foil", older),
				printing("galahad-s", "004-sir_galahad-b-s", "004", "Standard", older),
			},
		},
		{
			ID: "fireball", Name: "Fireball",
			Engine: Engine{Type: "Magic", Rarity: "Exceptional"},
			Printings: []Printing{
				printing("fireball-beta", "002-fireball-b-s", "002", "Standard", older),
				printing("fireball-fvl", "008-fireball-pd-s", "008", "Standard", newer),
			},
		},
		{
			ID: "vesuvius", Name: "Vesuvius",
			Engine: Engine{Type: TypeSite, Rarity: "Unique"},
			Printings: []Printing{
				printing("vesuvius-alpha", "001-vesuvius-b-s", "001", "Standard", older),
				printing("vesuvius-beta", "002-vesuvius-b-s", "002", "Standard", newer),
			},
		},
		{
			ID: "druid", Name: "Druid",
			Engine: Engine{Type: "Avatar", Back: &struct {
				Type string `json:"type"`
			}{Type: "Avatar"}},
			Printings: []Printing{printing("druid-bt", "004-druid-bt-s", "004", "Standard", older)},
		},
		{
			ID:     "bats",
			Name:   "Vampire Bats",
			Engine: Engine{Type: "Minion", Rarity: "Ordinary"},
			Printings: []Printing{
				printing("bats-beta", "002-vampire_bats-b-s", "002", "Standard", older),
			},
		},
		{
			ID:     "hydra",
			Name:   "Seirawan Hydra",
			Engine: Engine{Type: "Minion", Rarity: "Elite"},
			Printings: []Printing{
				printing("hydra-beta", "002-seirawan_hydra-b-s", "002", "Standard", older),
			},
		},
	})
}

func mustCard(t *testing.T, c *Catalog, name string) *Card {
	t.Helper()
	card, ok := c.Lookup(name)
	if !ok {
		t.Fatalf("card %q is not in the test catalog", name)
	}
	return card
}
