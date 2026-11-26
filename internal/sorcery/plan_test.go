package sorcery

import (
	"slices"
	"testing"
)

func TestBuildPlan(t *testing.T) {
	c := testCatalog(t)
	galahad := mustCard(t, c, "Sir Galahad")
	fireball := mustCard(t, c, "Fireball")
	vesuvius := mustCard(t, c, "Vesuvius")
	druid := mustCard(t, c, "Druid")
	lines := []Line{
		{Card: vesuvius, Printing: vesuvius.Printings[1], Group: "uniques", Want: 1},
		{Card: galahad, Printing: galahad.Printings[1], Group: "uniques", Want: 1},
		{
			Card:        galahad,
			Printing:    galahad.Printings[1],
			Group:       "errata",
			Want:        1,
			IgnoreOwned: true,
		},
		{
			Card:        fireball,
			Printing:    fireball.Printings[1],
			Group:       "errata",
			Want:        3,
			IgnoreOwned: true,
		},
		{Card: druid, Printing: druid.Printings[0], Group: "avatars", Want: 1},
	}
	owned := map[string]int{"galahad": 1, "fireball": 3, "druid": 1}
	printed := map[string]int{"fireball": 1}

	entries := BuildPlan(lines, owned, printed)

	type row struct {
		Name   string
		Groups int
		Want   int
		Print  int
	}
	got := make([]row, 0, len(entries))
	for _, e := range entries {
		got = append(got, row{Name: e.Name, Groups: len(e.Groups), Want: e.Want, Print: e.Print})
	}
	want := []row{
		// Owned, so the avatars group prints nothing.
		{Name: "Druid", Groups: 1, Want: 1, Print: 0},
		// Errata ignore the 3 owned copies, and 1 copy is printed already.
		{Name: "Fireball", Groups: 1, Want: 3, Print: 2},
		// Owned for uniques, but errata ignore the owned copy. The larger
		// want counts once, not twice.
		{Name: "Sir Galahad", Groups: 2, Want: 1, Print: 1},
		// Sites come after the spellbook cards.
		{Name: "Vesuvius", Groups: 1, Want: 1, Print: 1},
	}
	if !slices.Equal(got, want) {
		t.Errorf("BuildPlan =\n%v\nwant\n%v", got, want)
	}
	if !entries[0].DoubleFaced || entries[1].DoubleFaced {
		t.Error("BuildPlan marks only Druid as double-faced")
	}
	if entries[3].ImageURL != "https://d27a44hjr9gen3.cloudfront.net/cards/002-vesuvius-b-s.png" {
		t.Errorf("ImageURL = %s", entries[3].ImageURL)
	}

	if got, want := Total(entries), (Totals{Titles: 3, Spellbook: 3, Atlas: 1}); got != want {
		t.Errorf("Total = %+v, want %+v", got, want)
	}
}

func TestPlanRoundTrip(t *testing.T) {
	entries := []Entry{{CardID: "fireball", Name: "Fireball", Groups: []string{"errata"}, Print: 2}}
	data, err := MarshalPlan(entries)
	if err != nil {
		t.Fatalf("MarshalPlan: %v", err)
	}
	got, err := ParsePlan(data)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Fireball" || got[0].Print != 2 {
		t.Errorf("ParsePlan = %+v", got)
	}
}
