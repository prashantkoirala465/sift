package gmail

import "testing"

func TestParseFromHeader(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		wantAddr   string
		wantDomain string
	}{
		{"name and address", "Greenhouse <no-reply@greenhouse.io>", "no-reply@greenhouse.io", "greenhouse.io"},
		{"bare address", "recruiter@acme.com", "recruiter@acme.com", "acme.com"},
		{"quoted display name with comma", `"Doe, Jane" <jane@lever.co>`, "jane@lever.co", "lever.co"},
		{"mixed case domain lowercased", "HR <hr@ExAmple.COM>", "hr@ExAmple.COM", "example.com"},
		{"no angle brackets, extra whitespace", "  someone@company.io  ", "someone@company.io", "company.io"},
		{"malformed, no at sign", "not-an-email", "not-an-email", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			addr, domain := parseFromHeader(tc.header)
			if addr != tc.wantAddr {
				t.Errorf("address = %q, want %q", addr, tc.wantAddr)
			}
			if domain != tc.wantDomain {
				t.Errorf("domain = %q, want %q", domain, tc.wantDomain)
			}
		})
	}
}
