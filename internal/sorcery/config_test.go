package sorcery

import (
	"slices"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig([]byte(`
[pdf]
args = ["--paper_size", "tabloid"]

[[want]]
group = "errata"
cards = ["Fireball"]
quantities = { "Fireball" = 2 }
owned_printing_only = true
`))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !slices.Equal(cfg.PDF.Args, []string{"--paper_size", "tabloid"}) || len(cfg.Wants) != 1 {
		t.Fatalf("ParseConfig = %+v", cfg)
	}
	if w := cfg.Wants[0]; !w.OwnedPrintingOnly || w.Quantities["Fireball"] != 2 {
		t.Errorf("want = %+v", w)
	}
}

func TestParseConfigRejectsUnknownKeys(t *testing.T) {
	_, err := ParseConfig([]byte("[[want]]\ngroup = \"x\"\ncard = [\"Fireball\"]\n"))
	if err == nil || !strings.Contains(err.Error(), "unknown keys: want.card") {
		t.Errorf("ParseConfig error = %v, want the unknown key want.card", err)
	}
}

func TestParseOwned(t *testing.T) {
	c := testCatalog(t)
	owned, err := ParseOwned(c, []byte(`
# One playset of each set.
[[owned]]
group = "playsets"
sets = ["002", "006"]
rarities = ["Ordinary", "Elite"]

[[owned]]
group = "frost-mage-vs-lavamancer"
sets = ["008"]
quantity = 1
quantities = { "Fireball" = 2 }

[[owned]]
group = "named"
cards = ["Vampire Bats"]
sets = ["002", "006"]
quantity = 1
`))
	if err != nil {
		t.Fatalf("ParseOwned: %v", err)
	}
	want := map[string]int{
		// A playset from each set, then 1 more from the named group, which
		// picks one printing for its sets.
		"bats-beta":   4,
		"bats-gothic": 5,
		"hydra-beta":  2,
		// Only the quantities override.
		"fireball-fvl": 2,
	}
	if len(owned) != len(want) {
		t.Errorf("ParseOwned = %v, want %v", owned, want)
	}
	for id, q := range want {
		if owned[id] != q {
			t.Errorf("owned[%s] = %d, want %d", id, owned[id], q)
		}
	}
}

func TestParseOwnedErrors(t *testing.T) {
	c := testCatalog(t)
	if _, err := ParseOwned(
		c,
		[]byte("[[owned]]\ngroup = \"x\"\ncards = [\"Sir Galahadd\"]\n"),
	); err == nil {
		t.Error("ParseOwned accepted a name not in the catalog")
	}
	owned, err := ParseOwned(c, []byte("# Only comments.\n"))
	if err != nil || len(owned) != 0 {
		t.Errorf("ParseOwned of comments = %v, %v", owned, err)
	}
}
