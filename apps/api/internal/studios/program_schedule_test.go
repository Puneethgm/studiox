package studios

import "testing"

// Mirrors the real document's structure this was built from: "Week N"
// appearing twice in a row (a docx-conversion quirk this parser must
// tolerate), then day headers with Session/Progression & changes/Key focus
// blocks running until the next day or week header.
const sampleProgramDoc = `Intro
Program Notes: Test Program
Program Period: September - November 2026
Program Flow Weeks: 1 - 2
Some intro prose that mentions Week 3 and Tuesday in passing, which must
not be mistaken for a real header since it isn't on its own line.
Week 1
Week 1
Monday
Session: HIIT 411
Progression & changes: 1/8
Key focus:
Cardio HIIT focus paragraph one.
More key focus text on another line.
Tuesday
Session: Pump LB 276
Progression & changes: 1/3
Key focus: Lower body focus text.
Week 2
Week 2
Monday
Session: Hyper (M) 113
Progression & changes: 1/5
Key focus: Hypertrophy focus text.
`

func TestParseProgramSchedule(t *testing.T) {
	entries := ParseProgramSchedule(sampleProgramDoc)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}

	want := []ProgramSessionEntry{
		{WeekNumber: 1, DayOfWeek: 1, DayName: "Monday", SessionName: "HIIT 411", Progression: "1/8"},
		{WeekNumber: 1, DayOfWeek: 2, DayName: "Tuesday", SessionName: "Pump LB 276", Progression: "1/3"},
		{WeekNumber: 2, DayOfWeek: 1, DayName: "Monday", SessionName: "Hyper (M) 113", Progression: "1/5"},
	}
	for i, w := range want {
		got := entries[i]
		if got.WeekNumber != w.WeekNumber || got.DayOfWeek != w.DayOfWeek || got.DayName != w.DayName ||
			got.SessionName != w.SessionName || got.Progression != w.Progression {
			t.Errorf("entry %d = %+v, want %+v", i, got, w)
		}
		if got.KeyFocus == "" {
			t.Errorf("entry %d: expected non-empty KeyFocus", i)
		}
	}
}

func TestParseProgramSchedule_NotAProgramDoc(t *testing.T) {
	entries := ParseProgramSchedule("Just a regular FAQ document with no weekly structure at all.")
	if entries != nil {
		t.Fatalf("expected nil for non-matching text, got %+v", entries)
	}
}
