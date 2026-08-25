package match

import "strings"

// companySuffixes are stripped before comparing company names against
// email text -- "Acme Inc." should still match a subject line that just
// says "Acme".
var companySuffixes = []string{
	" inc.", " inc", " corp.", " corp", " corporation",
	" llc", " ltd.", " ltd", " co.", " company",
}

// minCompanyNameLength guards against false positives from short or
// generic names ("One", "Go") matching unrelated text.
const minCompanyNameLength = 4

// normalizeCompanyName lowercases and strips a trailing legal suffix. It
// returns "" for names too short to be a safe substring match.
func normalizeCompanyName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, suf := range companySuffixes {
		n = strings.TrimSuffix(n, suf)
	}
	n = strings.TrimSpace(n)

	if len(n) < minCompanyNameLength {
		return ""
	}
	return n
}
