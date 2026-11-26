package sorcery

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// Group selects cards by name, set, product and rarity, with a number of
// copies of each card. The want groups of proxies.toml and the owned groups
// of owned.toml share it.
type Group struct {
	// Name names the group in the plan.
	Name string `toml:"group"`
	// Note says why the group exists or what is left to check.
	Note string `toml:"note"`
	// Disabled leaves the group out.
	Disabled bool `toml:"disabled"`
	// Cards names the cards of the group. Every name must be in the catalog,
	// and every card must match the other selectors.
	Cards []string `toml:"cards"`
	// Sets keeps cards with a printing in one of these set codes, and picks
	// the printing from these sets.
	Sets []string `toml:"sets"`
	// Products keeps printings of these products, such as BoxTopper.
	Products []string `toml:"products"`
	// Rarities keeps cards of these rarities.
	Rarities []string `toml:"rarities"`
	// Quantity is the number of copies of each card. If it is not set, each
	// card gets a playset by rarity, as the deck limits allow: Ordinary 4,
	// Exceptional 3, Elite 2, and every other card 1.
	Quantity int `toml:"quantity"`
	// Quantities sets the copies of one card by name, over Quantity.
	Quantities map[string]int `toml:"quantities"`
	// Printings picks the printing of one card by name and printing slug.
	Printings map[string]string `toml:"printings"`
	// OwnedPrintingOnly subtracts only the owned copies from the set of the
	// picked printing. Errata use it, because older printings show the old
	// rules text. Owned groups ignore it.
	OwnedPrintingOnly bool `toml:"owned_printing_only"`
}

// Line is one card that one group selects.
type Line struct {
	Card              *Card
	Printing          Printing
	Group             string
	Quantity          int
	OwnedPrintingOnly bool
}

// decodeTOML decodes data into v. A key that v does not have is an error,
// so that a typo does not go unnoticed.
func decodeTOML(name string, data []byte, v any) error {
	md, err := toml.Decode(string(data), v)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, 0, len(undecoded))
		for _, k := range undecoded {
			keys = append(keys, k.String())
		}
		return fmt.Errorf("parse %s: unknown keys: %s", name, strings.Join(keys, ", "))
	}
	return nil
}

// Resolve turns the enabled groups into lines, one for each card of each
// group. It reports every name and printing that the catalog does not have.
func Resolve(catalog *Catalog, groups []Group) ([]Line, error) {
	var lines []Line
	var errs []error
	for i := range groups {
		g := &groups[i]
		if g.Disabled {
			continue
		}
		groupLines, err := resolveGroup(catalog, g)
		if err != nil {
			errs = append(errs, fmt.Errorf("group %q: %w", g.Name, err))
			continue
		}
		lines = append(lines, groupLines...)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return lines, nil
}

func resolveGroup(catalog *Catalog, g *Group) ([]Line, error) {
	cards, err := selectCards(catalog, g)
	if err != nil {
		return nil, err
	}
	lines := make([]Line, 0, len(cards))
	var errs []error
	for _, card := range cards {
		printing, err := pickPrinting(card, g)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		lines = append(lines, Line{
			Card:              card,
			Printing:          printing,
			Group:             g.Name,
			Quantity:          quantity(card, g),
			OwnedPrintingOnly: g.OwnedPrintingOnly,
		})
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return lines, nil
}

func selectCards(catalog *Catalog, g *Group) ([]*Card, error) {
	if len(g.Cards) == 0 && len(g.Sets) == 0 && len(g.Products) == 0 && len(g.Rarities) == 0 {
		return nil, errors.New(
			"set cards, sets, products or rarities: the group selects every card",
		)
	}
	var candidates []*Card
	if len(g.Cards) > 0 {
		var missing []string
		for _, name := range g.Cards {
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
		if matches(card, g) {
			cards = append(cards, card)
		} else if len(g.Cards) > 0 {
			unmatched = append(unmatched, card.Name)
		}
	}
	if len(unmatched) > 0 {
		return nil, fmt.Errorf(
			"cards outside the group's sets, products or rarities: %s",
			strings.Join(unmatched, ", "),
		)
	}
	return cards, nil
}

func matches(card *Card, g *Group) bool {
	if len(g.Rarities) > 0 && !slices.Contains(g.Rarities, card.Engine.Rarity) {
		return false
	}
	return slices.ContainsFunc(card.Printings, func(p Printing) bool { return inGroup(p, g) })
}

// inGroup reports whether a printing is in the group's sets and products.
func inGroup(p Printing, g *Group) bool {
	return (len(g.Sets) == 0 || slices.Contains(g.Sets, p.Set.Code)) &&
		(len(g.Products) == 0 || slices.Contains(g.Products, p.Meta.Product))
}

func quantity(card *Card, g *Group) int {
	if q, ok := g.Quantities[card.Name]; ok {
		return q
	}
	if g.Quantity > 0 {
		return g.Quantity
	}
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

// pickPrinting takes the printing that the group names for the card. Else it
// takes, from the group's sets and products, the Standard finish over Foil
// and Rainbow, then the newest printing, which shows the newest rules text.
func pickPrinting(card *Card, g *Group) (Printing, error) {
	if slug, ok := g.Printings[card.Name]; ok {
		i := slices.IndexFunc(card.Printings, func(p Printing) bool { return p.Slug == slug })
		if i < 0 {
			return Printing{}, fmt.Errorf("card %q has no printing %q", card.Name, slug)
		}
		return card.Printings[i], nil
	}
	var candidates []Printing
	for _, p := range card.Printings {
		if inGroup(p, g) {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return Printing{}, fmt.Errorf(
			"card %q has no printing in the group's sets and products",
			card.Name,
		)
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
