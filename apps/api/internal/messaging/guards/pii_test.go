package guards

import "testing"

func TestCheckPII_NoKnownFields_AnyPIIShapeFails(t *testing.T) {
	pack := DataPack{}
	got := CheckPII("Your date of birth on file is 12/05/1990.", pack)
	if got.Passed {
		t.Error("a date-shaped value with nothing on file to legitimately echo should fail")
	}
}

func TestCheckPII_NoKnownFields_NoMatchPasses(t *testing.T) {
	pack := DataPack{}
	got := CheckPII("Sure, I can help you book a class!", pack)
	if !got.Passed {
		t.Errorf("a reply with no PII-shaped content should pass, got: %+v", got.Violations)
	}
}

func TestCheckPII_KnownValue_Passes(t *testing.T) {
	pack := DataPack{KnownCustomerFields: map[string]string{"dob": "12/05/1990"}}
	got := CheckPII("Your date of birth on file is 12/05/1990.", pack)
	if !got.Passed {
		t.Errorf("echoing a known, on-file value should pass, got: %+v", got.Violations)
	}
}

func TestCheckPII_InventedValue_Fails(t *testing.T) {
	pack := DataPack{KnownCustomerFields: map[string]string{"dob": "12/05/1990"}}
	got := CheckPII("Your date of birth on file is 01/01/2000.", pack)
	if got.Passed {
		t.Error("a date that doesn't match any known on-file value should fail, even when some PII is on file")
	}
}

func TestCheckPII_NRICShape(t *testing.T) {
	pack := DataPack{}
	if CheckPII("Your NRIC on file is S1234567D.", pack).Passed {
		t.Error("an invented NRIC-shaped value should fail")
	}
	pack2 := DataPack{KnownCustomerFields: map[string]string{"nric": "S1234567D"}}
	if !CheckPII("Your NRIC on file is S1234567D.", pack2).Passed {
		t.Error("an NRIC matching a known on-file value should pass")
	}
}
