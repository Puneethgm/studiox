package messaging

import "testing"

func TestFormatMarkdownTablesAsPlainText(t *testing.T) {
	input := "Here's our schedule:\n\n" +
		"| Day | Session | Time |\n" +
		"|---|---|---|\n" +
		"| Monday | Yoga | 6:00 PM |\n" +
		"| Tuesday | Pilates | 7:00 AM |\n\n" +
		"Let me know if you'd like to book a trial!"

	got := formatMarkdownTablesAsPlainText(input)

	if got == input {
		t.Fatalf("expected table to be reformatted, got unchanged input")
	}
	for _, ch := range got {
		if ch == '|' {
			t.Fatalf("expected no pipe characters left in output, got: %q", got)
		}
	}

	want := "Here's our schedule:\n\n" +
		"Day / Session / Time:\n" +
		"Monday — Yoga — 6:00 PM\n" +
		"Tuesday — Pilates — 7:00 AM\n\n" +
		"Let me know if you'd like to book a trial!"
	if got != want {
		t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFormatMarkdownTablesAsPlainText_NoTable(t *testing.T) {
	input := "Hi! Thanks for reaching out — how can we help you today?"
	got := formatMarkdownTablesAsPlainText(input)
	if got != input {
		t.Fatalf("expected non-table text to pass through unchanged, got: %q", got)
	}
}

func TestFormatMarkdownTablesAsPlainText_MultipleTables(t *testing.T) {
	input := "Weekday:\n" +
		"| Day | Time |\n" +
		"|---|---|\n" +
		"| Mon | 6pm |\n\n" +
		"Weekend:\n" +
		"| Day | Time |\n" +
		"|---|---|\n" +
		"| Sat | 9am |\n"

	got := formatMarkdownTablesAsPlainText(input)
	want := "Weekday:\n" +
		"Day / Time:\n" +
		"Mon — 6pm\n\n" +
		"Weekend:\n" +
		"Day / Time:\n" +
		"Sat — 9am\n"
	if got != want {
		t.Fatalf("unexpected output:\ngot:  %q\nwant: %q", got, want)
	}
}
