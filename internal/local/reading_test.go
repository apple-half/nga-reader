package local

import (
	"math"
	"ngareader/internal/nga"
	"testing"
)

func TestReadingProgressMigratesAndSeparatesAccounts(t *testing.T) {
	old := Reading{Topic: nga.Topic{ID: 10, Replies: 5}, Page: 3}
	history := []Reading{old}
	r, ok := FindReading(history, 10, 123)
	if !ok || r.Page != 3 || r.Position != nil {
		t.Fatal("legacy page lost")
	}
	history = PutReading(history, Reading{Topic: old.Topic, Page: 3, UID: 123, Scoped: true, RepliesKnown: true, Position: &ReadingPosition{PID: 80, Floor: 42, Fraction: .4}})
	if r, ok := FindReading(history, 10, 456); ok && r.Position != nil {
		t.Fatal("another account received anchor")
	}
	r, ok = FindReading(history, 10, 123)
	if !ok || r.Position == nil || r.Position.PID != 80 {
		t.Fatal("anchor lost")
	}
	seen, changed := ReadStatus(history, nga.Topic{ID: 10, Replies: 6}, 123)
	if !seen || !changed {
		t.Fatal("new replies not marked")
	}
	seen, _ = ReadStatus(history, old.Topic, 456)
	if seen {
		t.Fatal("read marker leaked to another account")
	}
	seen, changed = ReadStatus(history, old.Topic, 123)
	if !seen || changed {
		t.Fatal("unchanged replies marked new")
	}
}

func TestReadingPositionsAreBoundedAndSurviveRestart(t *testing.T) {
	s := Defaults()
	for i := 1; i <= 120; i++ {
		s.History = PutReading(s.History, Reading{Topic: nga.Topic{ID: int64(i)}, Page: 2, UID: 123, Scoped: true, Position: &ReadingPosition{Floor: 21, Fraction: .5}})
	}
	if len(s.History) != 100 {
		t.Fatal("history unbounded")
	}
	dir := t.TempDir()
	if err := Save(dir, s); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := FindReading(got.History, 120, 123)
	if !ok || r.Position == nil || r.Position.Fraction != .5 {
		t.Fatal("restart lost position")
	}
	bad := Reading{Topic: nga.Topic{ID: 500}, Page: -3, UID: 123, Scoped: true, Position: &ReadingPosition{Floor: 2, Fraction: float32(math.NaN())}}
	got.History = PutReading(got.History, bad)
	r, _ = FindReading(got.History, 500, 123)
	if r.Page != 1 || r.Position != nil {
		t.Fatal("invalid saved position accepted")
	}
}
