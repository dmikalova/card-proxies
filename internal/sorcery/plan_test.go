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
		{Card: vesuvius, Printing: vesuvius.Printings[1], Group: "uniques", Quantity: 1},
		{Card: galahad, Printing: galahad.Printings[1], Group: "uniques", Quantity: 1},
		{Card: galahad, Printing: galahad.Printings[1], Group: "again", Quantity: 1},
		{
			Card:              fireball,
			Printing:          fireball.Printings[1],
			Group:             "errata",
			Quantity:          3,
			OwnedPrintingOnly: true,
		},
		{Card: druid, Printing: druid.Printings[0], Group: "avatars", Quantity: 1},
	}
	// 4 old Beta copies and 1 updated copy of Fireball.
	owned := map[string]int{"galahad-f": 1, "fireball-beta": 4, "fireball-fvl": 1, "druid-bt": 1}
	printed := map[string]int{"fireball": 1}

	entries := BuildPlan(lines, owned, printed)

	type row struct {
		Name   string
		Groups int
		Want   int
		Owned  int
		Print  int
	}
	got := make([]row, 0, len(entries))
	for _, e := range entries {
		got = append(
			got,
			row{Name: e.Name, Groups: len(e.Groups), Want: e.Want, Owned: e.Owned, Print: e.Print},
		)
	}
	want := []row{
		{Name: "Druid", Groups: 1, Want: 1, Owned: 1, Print: 0},
		// Only the updated copy counts, and 1 copy is printed already.
		{Name: "Fireball", Groups: 1, Want: 3, Owned: 1, Print: 1},
		// The owned foil counts for the card. Two groups count once.
		{Name: "Sir Galahad", Groups: 2, Want: 1, Owned: 1, Print: 0},
		// Sites come after the spellbook cards.
		{Name: "Vesuvius", Groups: 1, Want: 1, Owned: 0, Print: 1},
	}
	if !slices.Equal(got, want) {
		t.Errorf("BuildPlan =\n%v\nwant\n%v", got, want)
	}
	if !entries[0].DoubleFaced || entries[1].DoubleFaced {
		t.Error("BuildPlan marks only Druid as double-faced")
	}
	if entries[1].Printing != "008-fireball-pd-s" {
		t.Errorf("Fireball printing = %s", entries[1].Printing)
	}
	if entries[3].ImageURL != "https://d27a44hjr9gen3.cloudfront.net/cards/002-vesuvius-b-s.png" {
		t.Errorf("ImageURL = %s", entries[3].ImageURL)
	}
	if got, want := Total(entries), (Totals{Titles: 2, Spellbook: 1, Atlas: 1}); got != want {
		t.Errorf("Total = %+v, want %+v", got, want)
	}
}

func TestBuildPlanTakesTheLargerNeed(t *testing.T) {
	c := testCatalog(t)
	fireball := mustCard(t, c, "Fireball")
	lines := []Line{
		{Card: fireball, Printing: fireball.Printings[0], Group: "beta", Quantity: 3},
		{
			Card:              fireball,
			Printing:          fireball.Printings[1],
			Group:             "errata",
			Quantity:          3,
			OwnedPrintingOnly: true,
		},
	}
	entries := BuildPlan(lines, map[string]int{"fireball-beta": 3}, nil)

	if e := entries[0]; e.Print != 3 || e.Printing != "008-fireball-pd-s" || e.Owned != 0 {
		t.Errorf("entry = %+v, want 3 copies of the errata printing", e)
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
