package sorcery

import (
	"slices"
	"strings"
	"testing"
)

type resolved struct {
	Name     string
	Printing string
	Want     int
}

func summarize(lines []Line) []resolved {
	out := make([]resolved, 0, len(lines))
	for _, l := range lines {
		out = append(out, resolved{Name: l.Card.Name, Printing: l.Printing.Slug, Want: l.Want})
	}
	slices.SortFunc(out, func(a, b resolved) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name string
		want Want
		out  []resolved
	}{
		{
			name: "set and rarity pick the printing from the set",
			want: Want{Group: "uniques", Sets: []string{"002", "004"}, Rarity: "Unique"},
			out: []resolved{
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Want: 1},
				{Name: "Vesuvius", Printing: "002-vesuvius-b-s", Want: 1},
			},
		},
		{
			name: "named cards take the newest Standard printing",
			want: Want{Group: "errata", Cards: []string{"fireball", "Sir Galahad"}, Quantity: 2},
			out: []resolved{
				{Name: "Fireball", Printing: "008-fireball-pd-s", Want: 2},
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Want: 2},
			},
		},
		{
			name: "playset follows rarity and quantities override it",
			want: Want{
				Group: "errata",
				Cards: []string{
					"Vampire Bats",
					"Seirawan Hydra",
					"Fireball",
					"Druid",
					"Sir Galahad",
				},
				Playset:    true,
				Quantities: map[string]int{"Sir Galahad": 3},
			},
			out: []resolved{
				{Name: "Druid", Printing: "004-druid-bt-s", Want: 1},
				{Name: "Fireball", Printing: "008-fireball-pd-s", Want: 3},
				{Name: "Seirawan Hydra", Printing: "002-seirawan_hydra-b-s", Want: 2},
				{Name: "Sir Galahad", Printing: "004-sir_galahad-b-s", Want: 3},
				{Name: "Vampire Bats", Printing: "002-vampire_bats-b-s", Want: 4},
			},
		},
		{
			name: "printings pick a printing by slug",
			want: Want{
				Group:     "pick",
				Cards:     []string{"Fireball"},
				Printings: map[string]string{"Fireball": "002-fireball-b-s"},
			},
			out: []resolved{{Name: "Fireball", Printing: "002-fireball-b-s", Want: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines, err := Resolve(testCatalog(t), []Want{tt.want})
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
		want    Want
		message string
	}{
		{
			name:    "no selector",
			want:    Want{Group: "all"},
			message: "the group selects every card",
		},
		{
			name:    "names not in the catalog",
			want:    Want{Group: "typo", Cards: []string{"Vampire Bat", "Fireball", "Lancee"}},
			message: "cards not in the catalog: Vampire Bat, Lancee",
		},
		{
			name:    "named card outside the sets",
			want:    Want{Group: "beta", Cards: []string{"Sir Galahad"}, Sets: []string{"002"}},
			message: "cards outside the group's sets or rarity: Sir Galahad",
		},
		{
			name: "printing slug not on the card",
			want: Want{
				Group:     "pick",
				Cards:     []string{"Fireball"},
				Printings: map[string]string{"Fireball": "009-fireball-b-s"},
			},
			message: `card "Fireball" has no printing "009-fireball-b-s"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Resolve(testCatalog(t), []Want{tt.want})
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Errorf("Resolve error = %v, want it to contain %q", err, tt.message)
			}
		})
	}
}

func TestResolveSkipsDisabled(t *testing.T) {
	lines, err := Resolve(
		testCatalog(t),
		[]Want{{Group: "off", Disabled: true, Cards: []string{"Not A Card"}}},
	)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("Resolve = %d lines, want 0", len(lines))
	}
}

func TestParseConfigRejectsUnknownFields(t *testing.T) {
	_, err := ParseConfig([]byte("wants:\n  - group: x\n    card: [Fireball]\n"))
	if err == nil {
		t.Fatal("ParseConfig accepted the unknown field card")
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig(
		[]byte(
			"pdf:\n  args: [--paper_size, tabloid]\nwants:\n  - group: x\n    cards: [Fireball]\n    ignore_owned: true\n",
		),
	)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if !slices.Equal(cfg.PDF.Args, []string{"--paper_size", "tabloid"}) || len(cfg.Wants) != 1 ||
		!cfg.Wants[0].IgnoreOwned {
		t.Errorf("ParseConfig = %+v", cfg)
	}
}
