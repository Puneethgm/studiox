package messaging

import "testing"

func TestParseBroadcastContactRows_FuzzyHeaders(t *testing.T) {
	sheet := [][]string{
		{"Client Name", "Phone Number", "Email Address"},
		{"Sharon Shen", "91234567", "sharon@example.com"},
		{"", "", ""}, // blank trailing row — must be skipped
		{"Lowell Chua", "+6598765432", ""},
	}
	rows, err := ParseBroadcastContactRows(sheet)
	if err != nil {
		t.Fatalf("ParseBroadcastContactRows() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (blank row should be skipped): %+v", len(rows), rows)
	}
	if rows[0].Name != "Sharon Shen" || rows[0].Phone != "91234567" || rows[0].Email != "sharon@example.com" {
		t.Errorf("row[0] = %+v", rows[0])
	}
	if rows[1].Name != "Lowell Chua" || rows[1].Phone != "+6598765432" {
		t.Errorf("row[1] = %+v", rows[1])
	}
}

func TestParseBroadcastContactRows_MissingPhoneColumn(t *testing.T) {
	sheet := [][]string{
		{"Name", "Email"},
		{"Sharon", "sharon@example.com"},
	}
	_, err := ParseBroadcastContactRows(sheet)
	if err == nil {
		t.Fatal("ParseBroadcastContactRows() want error for missing phone column, got nil")
	}
}

func TestHasCountryCode(t *testing.T) {
	cases := map[string]bool{
		"+6591234567": true,
		"91234567":    false,
		"091234567":   false,
		"":            false,
		" +65 1234":   true,
	}
	for phone, want := range cases {
		if got := hasCountryCode(phone); got != want {
			t.Errorf("hasCountryCode(%q) = %v, want %v", phone, got, want)
		}
	}
}

func TestApplyCountryCode(t *testing.T) {
	cases := []struct{ phone, code, want string }{
		{"91234567", "65", "+6591234567"},
		{"091234567", "65", "+6591234567"},  // leading trunk zero stripped
		{"9123 4567", "65", "+6591234567"},  // spaces stripped
		{"9123-4567", "+65", "+6591234567"}, // code itself may have a '+'
	}
	for _, c := range cases {
		if got := applyCountryCode(c.phone, c.code); got != c.want {
			t.Errorf("applyCountryCode(%q, %q) = %q, want %q", c.phone, c.code, got, c.want)
		}
	}
}
