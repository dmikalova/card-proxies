package sorcery

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
)

// Entry is one card of a plan: what the wants ask for, what is owned and
// already printed, and how many copies the run prints.
type Entry struct {
	CardID      string   `json:"card_id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Printing    string   `json:"printing"`
	ImageURL    string   `json:"image_url"`
	Groups      []string `json:"groups"`
	Want        int      `json:"want"`
	Owned       int      `json:"owned"`
	Printed     int      `json:"printed"`
	Print       int      `json:"print"`
	DoubleFaced bool     `json:"double_faced,omitempty"`
}

// Atlas reports whether the card takes the atlas card back.
func (e *Entry) Atlas() bool {
	return e.Type == TypeSite
}

// BuildPlan merges the lines of each card and subtracts what is owned and
// already printed. A card in two groups gets the larger of the two wants,
// not their sum. A group with IgnoreOwned does not subtract owned copies.
// The plan lists spellbook cards before atlas cards, each by name.
func BuildPlan(lines []Line, owned, printed map[string]int) []Entry {
	byCard := make(map[string]*Entry)
	var order []*Entry
	for _, line := range lines {
		card := line.Card
		need := line.Want
		if !line.IgnoreOwned {
			need -= owned[card.ID]
		}
		e, ok := byCard[card.ID]
		if !ok {
			e = &Entry{
				CardID:      card.ID,
				Name:        card.Name,
				Type:        card.Engine.Type,
				Printing:    line.Printing.Slug,
				ImageURL:    line.Printing.ImageURL(),
				Owned:       owned[card.ID],
				Printed:     printed[card.ID],
				DoubleFaced: card.Engine.Back != nil,
			}
			byCard[card.ID] = e
			order = append(order, e)
		}
		e.Groups = append(e.Groups, line.Group)
		e.Want = max(e.Want, line.Want)
		e.Print = max(e.Print, need)
	}
	entries := make([]Entry, 0, len(order))
	for _, e := range order {
		e.Print = max(e.Print-e.Printed, 0)
		entries = append(entries, *e)
	}
	slices.SortFunc(entries, func(a, b Entry) int {
		return cmp.Or(compareBool(a.Atlas(), b.Atlas()), cmp.Compare(a.Name, b.Name))
	})
	return entries
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	default:
		return -1
	}
}

// Totals counts the cards and copies that a plan prints.
type Totals struct {
	Titles    int
	Spellbook int
	Atlas     int
}

// Total counts the cards and copies that the plan prints.
func Total(entries []Entry) Totals {
	var t Totals
	for i := range entries {
		e := &entries[i]
		if e.Print == 0 {
			continue
		}
		t.Titles++
		if e.Atlas() {
			t.Atlas += e.Print
		} else {
			t.Spellbook += e.Print
		}
	}
	return t
}

// MarshalPlan writes the plan as indented JSON.
func MarshalPlan(entries []Entry) ([]byte, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the plan: %w", err)
	}
	return append(data, '\n'), nil
}

// ParsePlan reads a run's plan.json.
func ParsePlan(data []byte) ([]Entry, error) {
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse the plan: %w", err)
	}
	return entries, nil
}
