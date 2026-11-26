package sorcery

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Ledger is printed.json: every run that was printed and cut.
type Ledger struct {
	Runs []Run `json:"runs"`
}

// Run is one printed run.
type Run struct {
	ID    string    `json:"id"`
	Cards []RunCard `json:"cards"`
}

// RunCard is the number of copies of one card in a run.
type RunCard struct {
	CardID   string `json:"card_id"`
	Name     string `json:"name"`
	Printing string `json:"printing"`
	Quantity int    `json:"quantity"`
}

// ParseLedger reads printed.json. Empty data is an empty ledger.
func ParseLedger(data []byte) (*Ledger, error) {
	var l Ledger
	if len(data) == 0 {
		return &l, nil
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("parse the ledger: %w", err)
	}
	return &l, nil
}

// Marshal writes the ledger as indented JSON.
func (l *Ledger) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode the ledger: %w", err)
	}
	return append(data, '\n'), nil
}

// PrintedByCard adds up the printed copies of each card, keyed by card ID.
func (l *Ledger) PrintedByCard() map[string]int {
	byCard := make(map[string]int)
	for _, run := range l.Runs {
		for _, c := range run.Cards {
			byCard[c.CardID] += c.Quantity
		}
	}
	return byCard
}

// Record adds the printed entries of a run's plan. It refuses a run ID that
// the ledger already holds, so that a run is not counted twice.
func (l *Ledger) Record(id string, entries []Entry) error {
	if slices.ContainsFunc(l.Runs, func(r Run) bool { return r.ID == id }) {
		return fmt.Errorf("run %s is already in the ledger", id)
	}
	run := Run{ID: id}
	for i := range entries {
		e := &entries[i]
		if e.Print == 0 {
			continue
		}
		run.Cards = append(
			run.Cards,
			RunCard{CardID: e.CardID, Name: e.Name, Printing: e.Printing, Quantity: e.Print},
		)
	}
	if len(run.Cards) == 0 {
		return fmt.Errorf("run %s prints no cards", id)
	}
	l.Runs = append(l.Runs, run)
	return nil
}
