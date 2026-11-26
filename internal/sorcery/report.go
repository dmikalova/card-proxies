package sorcery

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
)

// Report is what a plan or a staged run prints to the terminal.
type Report struct {
	Entries []Entry
	// Images holds the size of each staged image by printing slug. It is
	// empty for a plan, which downloads nothing.
	Images map[string]ImageInfo
	// OwnedFrom names the file of the owned groups.
	OwnedFrom string
	// Staged reports whether the run is staged, and Backs whether it has
	// card backs.
	Staged bool
	Backs  bool
}

// Write prints the cards to print, the totals and the warnings.
func (r *Report) Write(w io.Writer) error {
	var b strings.Builder
	if r.OwnedFrom == "" {
		b.WriteString("Owned: nothing. owned.toml does not exist.\n\n")
	} else {
		fmt.Fprintf(&b, "Owned: from %s.\n\n", r.OwnedFrom)
	}
	// The rows go to a strings.Builder first, which cannot fail, and then
	// through the tabwriter in one write.
	var rows strings.Builder
	rows.WriteString("CARD\tTYPE\tPRINTING\tGROUPS\tWANT\tOWNED\tPRINTED\tPRINT\tPPI\n")
	complete := 0
	var lowRes, doubleFaced []string
	for i := range r.Entries {
		e := &r.Entries[i]
		if e.Print == 0 {
			complete++
			continue
		}
		ppi := "-"
		if info, ok := r.Images[e.Printing]; ok {
			ppi = strconv.Itoa(info.PPI())
			if info.PPI() < MinPPI {
				lowRes = append(lowRes, fmt.Sprintf("%s (%d PPI)", e.Name, info.PPI()))
			}
		}
		if e.DoubleFaced {
			doubleFaced = append(doubleFaced, e.Name)
		}
		fmt.Fprintf(
			&rows,
			"%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%s\n",
			e.Name,
			e.Type,
			e.Printing,
			strings.Join(e.Groups, ", "),
			e.Want,
			e.Owned,
			e.Printed,
			e.Print,
			ppi,
		)
	}
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	if _, err := io.WriteString(tw, rows.String()); err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	t := Total(r.Entries)
	fmt.Fprintf(
		&b,
		"\nCards to print: %d. Spellbook copies: %d. Atlas copies: %d.\n",
		t.Titles,
		t.Spellbook,
		t.Atlas,
	)
	fmt.Fprintf(&b, "Cards already owned or printed: %d.\n", complete)
	if len(lowRes) > 0 {
		fmt.Fprintf(
			&b,
			"\nLow resolution, below %d PPI at card size:\n  %s\n",
			MinPPI,
			strings.Join(lowRes, "\n  "),
		)
	}
	if len(doubleFaced) > 0 {
		fmt.Fprintf(
			&b,
			"\nDouble-faced cards. The run prints the front face only. The back face image is <printing>-back.png:\n  %s\n",
			strings.Join(doubleFaced, "\n  "),
		)
	}
	if r.Staged && !r.Backs {
		b.WriteString(
			"\nNo card backs: the PDF prints fronts only. Put spellbook.png and atlas.png in backs/sorcery/ to print backs.\n",
		)
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("write the report: %w", err)
	}
	return nil
}
