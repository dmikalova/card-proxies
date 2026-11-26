package sorcery

import (
	"slices"
	"strings"
	"testing"
)

type resolved struct {
	Name     string
	Printing string
	Quantity int
}

func summarize(lines []Line) []resolved {
	out := make([]resolved, 0, len(lines))
	for _, l := range lines {
		out = append(
			out,
			resolved{Name: l.Card.Name, Printing: l.Printing.Slug, Quantity: l.Quantity},
		)
	}
	slices.SortFunc(out, func(a, b resolved) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name  string
		group Group
		out   []resolved
	}{
		{
			name: "sets and rarities pick the printing from the sets",
			group: Group{
				Name:     "uniques",
				Sets:     []string{"002", "004"},
				Rarities: []string{"Unique"},
			},
			out: []resolved{
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Quantity: 1},
				{Name: "Vesuvius", Printing: "002-vesuvius-b-s", Quantity: 1},
			},
		},
		{
			name:  "named cards take the newest Standard printing",
			group: Group{Name: "pick", Cards: []string{"fireball", "Sir Galahad"}, Quantity: 2},
			out: []resolved{
				{Name: "Fireball", Printing: "008-fireball-pd-s", Quantity: 2},
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Quantity: 2},
			},
		},
		{
			name: "no quantity gives a playset by rarity, and quantities override it",
			group: Group{
				Name: "playsets",
				Cards: []string{
					"Vampire Bats",
					"Seirawan Hydra",
					"Fireball",
					"Druid",
					"Sir Galahad",
				},
				Quantities: map[string]int{"Sir Galahad": 3},
			},
			out: []resolved{
				{Name: "Druid", Printing: "004-druid-bt-s", Quantity: 1},
				{Name: "Fireball", Printing: "008-fireball-pd-s", Quantity: 3},
				{Name: "Seirawan Hydra", Printing: "002-seirawan_hydra-b-s", Quantity: 2},
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Quantity: 3},
				{Name: "Vampire Bats", Printing: "006-vampire_bats-b-s", Quantity: 4},
			},
		},
		{
			name:  "products select box toppers",
			group: Group{Name: "toppers", Products: []string{"BoxTopper"}},
			out:   []resolved{{Name: "Druid", Printing: "004-druid-bt-s", Quantity: 1}},
		},
		{
			name: "printings pick a printing by slug",
			group: Group{
				Name:      "pick",
				Cards:     []string{"Fireball"},
				Printings: map[string]string{"Fireball": "002-fireball-b-s"},
			},
			out: []resolved{{Name: "Fireball", Printing: "002-fireball-b-s", Quantity: 3}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := Resolve(testCatalog(t), []Group{tt.group})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got := summarize(lines); !slices.Equal(got, tt.out) {
				t.Errorf("Resolve =\n%v\nwant\n%v", got, tt.out)
			}
		})
	}
}

func TestResolveErrors(t *testing.T) {
	tests := []struct {
		name    string
		group   Group
		message string
	}{
		{
			name:    "no selector",
			group:   Group{Name: "all"},
			message: "the group selects every card",
		},
		{
			name:    "names not in the catalog",
			group:   Group{Name: "typo", Cards: []string{"Vampire Bat", "Fireball", "Lancee"}},
			message: "cards not in the catalog: Vampire Bat, Lancee",
		},
		{
			name:    "named card outside the sets",
			group:   Group{Name: "beta", Cards: []string{"Sir Galahad"}, Sets: []string{"002"}},
			message: "cards outside the group's sets, products or rarities: Sir Galahad",
		},
		{
			name: "printing slug not on the card",
			group: Group{
				Name:      "pick",
				Cards:     []string{"Fireball"},
				Printings: map[string]string{"Fireball": "009-fireball-b-s"},
			},
			message: `card "Fireball" has no printing "009-fireball-b-s"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(testCatalog(t), []Group{tt.group})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Errorf("Resolve error = %v, want it to contain %q", err, tt.message)
			}
		})
	}
}

func TestResolveSkipsDisabled(t *testing.T) {
	lines, err := Resolve(
		testCatalog(t),
		[]Group{{Name: "off", Disabled: true, Cards: []string{"Not A Card"}}},
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("Resolve = %d lines, want 0", len(lines))
	}
}
