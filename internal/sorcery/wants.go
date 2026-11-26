package sorcery

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Config is the hand-edited proxies.yaml of the game: the PDF settings and
// the wants.
type Config struct {
	PDF   PDFConfig `yaml:"pdf"`
	Wants []Want    `yaml:"wants"`
}

// PDFConfig holds the create_pdf.py flags of every run. The card directories
// and the output path are added for each run.
type PDFConfig struct {
	Args []string `yaml:"args"`
}

// Want is one group of cards to proxy. A group names its cards, selects them
// by set and rarity, or both.
type Want struct {
	// Group names the group in the plan.
	Group string `yaml:"group"`
	// Note says why the group exists or what is left to check.
	Note string `yaml:"note"`
	// Disabled leaves the group out of the plan.
	Disabled bool `yaml:"disabled"`
	// Cards names the cards of the group. Every name must be in the catalog.
	Cards []string `yaml:"cards"`
	// Sets keeps cards with a printing in one of these set codes, and picks
	// the printing from these sets.
	Sets []string `yaml:"sets"`
	// Rarity keeps cards of this rarity.
	Rarity string `yaml:"rarity"`
	// Quantity is the number of copies of each card. It is 1 if it is not
	// set.
	Quantity int `yaml:"quantity"`
	// Playset sets the quantity of each card from its rarity, as the deck
	// limits do: Ordinary 4, Exceptional 3, Elite 2, every other card 1.
	Playset bool `yaml:"playset"`
	// Quantities sets the quantity of one card by name, over Quantity and
	// Playset.
	Quantities map[string]int `yaml:"quantities"`
	// Printings picks the printing of one card by name and printing slug.
	Printings map[string]string `yaml:"printings"`
	// IgnoreOwned prints the cards even if the collection holds them, such
	// as errata that the owned printings do not show.
	IgnoreOwned bool `yaml:"ignore_owned"`
}

// Line is one card that one want group asks for.
type Line struct {
	Card        *Card
	Printing    Printing
	Group       string
	Want        int
	IgnoreOwned bool
}

// ParseConfig reads proxies.yaml. A field that the config does not know is an
// error, so that a typo does not go unnoticed.
func ParseConfig(data []byte) (*Config, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var c Config
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse the config: %w", err)
	}
	return &c, nil
}

// Resolve turns the enabled want groups into lines, one for each card of each
// group. It reports every name and printing that the catalog does not have.
func Resolve(catalog *Catalog, wants []Want) ([]Line, error) {
	var lines []Line
	var errs []error
	for i := range wants {
		w := &wants[i]
		if w.Disabled {
			continue
		}
		groupLines, err := resolveWant(catalog, w)
		if err != nil {
			errs = append(errs, fmt.Errorf("want group %q: %w", w.Group, err))
			continue
		}
		lines = append(lines, groupLines...)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return lines, nil
}

func resolveWant(catalog *Catalog, w *Want) ([]Line, error) {
	cards, err := selectCards(catalog, w)
	if err != nil {
		return nil, err
	}
	lines := make([]Line, 0, len(cards))
	var errs []error
	for _, card := range cards {
		printing, err := pickPrinting(card, w)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		lines = append(lines, Line{
			Card:        card,
			Printing:    printing,
			Group:       w.Group,
			Want:        quantity(card, w),
			IgnoreOwned: w.IgnoreOwned,
		})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return lines, nil
}

func selectCards(catalog *Catalog, w *Want) ([]*Card, error) {
	if len(w.Cards) == 0 && len(w.Sets) == 0 && w.Rarity == "" {
		return nil, errors.New("set cards, sets or rarity: the group selects every card")
	}
	var candidates []*Card
	if len(w.Cards) > 0 {
		var missing []string
		for _, name := range w.Cards {
			card, ok := catalog.Lookup(name)
			if !ok {
				missing = append(missing, name)
				continue
			}
			candidates = append(candidates, card)
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("cards not in the catalog: %s", strings.Join(missing, ", "))
		}
	} else {
		for i := range catalog.Cards {
			candidates = append(candidates, &catalog.Cards[i])
		}
	}
	var cards []*Card
	var unmatched []string
	for _, card := range candidates {
		if matches(card, w) {
			cards = append(cards, card)
		} else if len(w.Cards) > 0 {
			unmatched = append(unmatched, card.Name)
		}
	}
	if len(unmatched) > 0 {
		return nil, fmt.Errorf(
			"cards outside the group's sets or rarity: %s",
			strings.Join(unmatched, ", "),
		)
	}
	return cards, nil
}

func matches(card *Card, w *Want) bool {
	if w.Rarity != "" && card.Engine.Rarity != w.Rarity {
		return false
	}
	if len(w.Sets) == 0 {
		return true
	}
	return slices.ContainsFunc(card.Printings, func(p Printing) bool {
		return slices.Contains(w.Sets, p.Set.Code)
	})
}

func quantity(card *Card, w *Want) int {
	if q, ok := w.Quantities[card.Name]; ok {
		return q
	}
	if w.Playset {
		switch card.Engine.Rarity {
		case "Ordinary":
			return 4
		case "Exceptional":
			return 3
		case "Elite":
			return 2
		default:
			return 1
		}
	}
	return max(w.Quantity, 1)
}

// pickPrinting takes the printing that the group names for the card. Else it
// takes, from the group's sets, the Standard finish over Foil and Rainbow,
// then the newest printing, which shows the newest rules text.
func pickPrinting(card *Card, w *Want) (Printing, error) {
	if slug, ok := w.Printings[card.Name]; ok {
		i := slices.IndexFunc(card.Printings, func(p Printing) bool { return p.Slug == slug })
		if i < 0 {
			return Printing{}, fmt.Errorf("card %q has no printing %q", card.Name, slug)
		}
		return card.Printings[i], nil
	}
	var candidates []Printing
	for _, p := range card.Printings {
		if len(w.Sets) == 0 || slices.Contains(w.Sets, p.Set.Code) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return Printing{}, fmt.Errorf("card %q has no printing in the group's sets", card.Name)
	}
	return slices.MinFunc(candidates, comparePrintings), nil
}

func comparePrintings(a, b Printing) int {
	return cmp.Or(
		cmp.Compare(finishRank(a.Meta.Finish), finishRank(b.Meta.Finish)),
		b.PrintedAt.Compare(a.PrintedAt),
		cmp.Compare(a.Slug, b.Slug),
	)
}

func finishRank(finish string) int {
	switch finish {
	case "Standard":
		return 0
	case "Foil":
		return 1
	default:
		return 2
	}
}
