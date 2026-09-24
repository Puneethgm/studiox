package billing

import "testing"

func TestRecurringForCycle(t *testing.T) {
	cases := []struct {
		cycle         string
		wantInterval  string
		wantCount     int64
		wantRecurring bool
	}{
		{"monthly", "month", 1, true},
		{"yearly", "year", 1, true},
		{"quarterly", "month", 3, true},
		{"semi_annually", "month", 6, true},
		{"every_2_months", "month", 2, true},
		{"daily", "day", 1, true},
		{"weekly", "week", 1, true},
		{"biweekly", "week", 2, true},
		{"every_4_weeks", "week", 4, true},
		{"every_6_weeks", "week", 6, true},
		{"one_time", "", 0, false},
		{"", "month", 1, true},
		{"unknown_legacy_value", "month", 1, true},
	}

	for _, tc := range cases {
		t.Run(tc.cycle, func(t *testing.T) {
			interval, count, recurring := RecurringForCycle(tc.cycle)
			if interval != tc.wantInterval || count != tc.wantCount || recurring != tc.wantRecurring {
				t.Errorf("RecurringForCycle(%q) = (%q, %d, %v), want (%q, %d, %v)",
					tc.cycle, interval, count, recurring, tc.wantInterval, tc.wantCount, tc.wantRecurring)
			}
		})
	}
}

func TestValidCycles(t *testing.T) {
	want := []string{
		"monthly", "yearly", "one_time", "daily", "weekly", "biweekly", "every_4_weeks", "every_6_weeks",
		"quarterly", "semi_annually", "every_2_months", "custom",
	}
	for _, c := range want {
		if !ValidCycles[c] {
			t.Errorf("expected %q to be a valid cycle", c)
		}
	}
	if ValidCycles["not_a_real_cycle"] {
		t.Error("expected unknown cycle to be invalid")
	}
}

func TestValidateCustomInterval(t *testing.T) {
	cases := []struct {
		name    string
		unit    string
		count   int64
		wantErr bool
	}{
		{"valid week", "week", 5, false},
		{"valid month", "month", 10, false},
		{"valid day at max", "day", 365, false},
		{"valid year at max", "year", 3, false},
		{"bad unit", "fortnight", 2, true},
		{"zero count", "week", 0, true},
		{"negative count", "month", -1, true},
		{"month over max", "month", 13, true},
		{"year over max", "year", 4, true},
		{"week over max", "week", 53, true},
		{"day over max", "day", 366, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateCustomInterval(tc.unit, tc.count)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateCustomInterval(%q, %d) err = %v, wantErr = %v", tc.unit, tc.count, err, tc.wantErr)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	interval, count, recurring := Resolve("custom", "week", 5)
	if interval != "week" || count != 5 || !recurring {
		t.Errorf("Resolve(custom, week, 5) = (%q, %d, %v), want (week, 5, true)", interval, count, recurring)
	}

	// Invalid custom interval falls back to monthly rather than breaking checkout.
	interval, count, recurring = Resolve("custom", "fortnight", 2)
	if interval != "month" || count != 1 || !recurring {
		t.Errorf("Resolve(custom, fortnight, 2) = (%q, %d, %v), want fallback (month, 1, true)", interval, count, recurring)
	}

	// Non-custom cycles ignore the custom args entirely.
	interval, count, recurring = Resolve("yearly", "week", 5)
	if interval != "year" || count != 1 || !recurring {
		t.Errorf("Resolve(yearly, ...) = (%q, %d, %v), want (year, 1, true)", interval, count, recurring)
	}
}
