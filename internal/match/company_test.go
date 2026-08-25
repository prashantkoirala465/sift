package match

import "testing"

func TestNormalizeCompanyName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"strips Inc.", "Acme Inc.", "acme"},
		{"strips Corp", "Globex Corp", "globex"},
		{"strips LLC", "Initech LLC", "initech"},
		{"lowercases plain name", "Waybill", "waybill"},
		{"too short after normalizing", "Go Inc.", ""},
		{"too short bare", "Up", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeCompanyName(tc.in); got != tc.want {
				t.Errorf("normalizeCompanyName(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
