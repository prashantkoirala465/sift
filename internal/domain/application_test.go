package domain

import "testing"

func TestSourceValid(t *testing.T) {
	valid := []Source{SourceLinkedIn, SourceReferral, SourceCompanySite, SourceJobBoard, SourceOther}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}

	invalid := []Source{"", "LinkedIn", "unknown"}
	for _, s := range invalid {
		if s.Valid() {
			t.Errorf("%q should not be valid", s)
		}
	}
}
