package sorcery

import "testing"

func TestLedger(t *testing.T) {
	l, err := ParseLedger(nil)
	if err != nil {
		t.Fatalf("ParseLedger(nil): %v", err)
	}
	entries := []Entry{
		{CardID: "fireball", Name: "Fireball", Printing: "008-fireball-pd-s", Print: 2},
		{CardID: "done", Name: "Done", Print: 0},
	}
	if err := l.Record("r1", entries); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if err := l.Record("r1", entries); err == nil {
		t.Error("Record accepted run r1 twice")
	}
	if err := l.Record("r2", entries[1:]); err == nil {
		t.Error("Record accepted a run that prints no cards")
	}
	if err := l.Record("r3", entries[:1]); err != nil {
		t.Fatalf("Record r3: %v", err)
	}

	data, err := l.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	back, err := ParseLedger(data)
	if err != nil {
		t.Fatalf("ParseLedger: %v", err)
	}
	if got := back.PrintedByCard(); got["fireball"] != 4 || len(got) != 1 {
		t.Errorf("PrintedByCard = %v, want fireball 4", got)
	}
	if len(back.Runs[0].Cards) != 1 {
		t.Errorf("run r1 = %+v, want only the printed card", back.Runs[0])
	}
}
