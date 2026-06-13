package cmd

import "testing"

func TestValidateStatus(t *testing.T) {
	for _, s := range []string{"considering", "todo", "in_flight", "completed", "tabled"} {
		if err := validateStatus(s); err != nil {
			t.Errorf("validateStatus(%q) = %v, want nil", s, err)
		}
	}

	// blocked is no longer a status; it must be rejected.
	if err := validateStatus("blocked"); err == nil {
		t.Error("validateStatus(\"blocked\") = nil, want error (blocked is a flag now)")
	}

	err := validateStatus("bogus")
	if err == nil {
		t.Fatal("validateStatus(\"bogus\") = nil, want error")
	}
	for _, want := range []string{"bogus", "considering", "todo", "in_flight", "completed", "tabled"} {
		if !contains(err.Error(), want) {
			t.Errorf("validateStatus error %q missing %q", err.Error(), want)
		}
	}
}

func TestValidateFormat(t *testing.T) {
	if err := validateFormat("brief"); err != nil {
		t.Errorf("validateFormat(\"brief\") = %v, want nil", err)
	}
	if err := validateFormat("full"); err != nil {
		t.Errorf("validateFormat(\"full\") = %v, want nil", err)
	}
	err := validateFormat("xml")
	if err == nil {
		t.Fatal("validateFormat(\"xml\") = nil, want error")
	}
	for _, want := range []string{"xml", "brief", "full"} {
		if !contains(err.Error(), want) {
			t.Errorf("validateFormat error %q missing %q", err.Error(), want)
		}
	}
}

func TestParseID(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"42", 42, false},
		{"#42", 42, false},
		{"abc", 0, true},
		{"", 0, true},
		{"0", 0, true},
		{"-5", 0, true},
		{"4x", 0, true},
	}
	for _, c := range cases {
		got, err := parseID(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseID(%q) = %d, nil; want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseID(%q) = error %v; want %d", c.in, err, c.want)
		}
		if got != c.want {
			t.Errorf("parseID(%q) = %d; want %d", c.in, got, c.want)
		}
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
