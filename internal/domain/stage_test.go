package domain

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		name string
		from Stage
		to   Stage
		want bool
	}{
		{"applied to screening", StageApplied, StageScreening, true},
		{"applied to offer skips stages", StageApplied, StageOffer, false},
		{"applied to interview skips screening", StageApplied, StageInterview, false},
		{"screening to interview", StageScreening, StageInterview, true},
		{"interview to next interview round", StageInterview, StageInterview, true},
		{"interview to offer", StageInterview, StageOffer, true},
		{"offer to accepted", StageOffer, StageAccepted, true},
		{"offer to declined", StageOffer, StageDeclined, true},
		{"offer to interview goes backwards", StageOffer, StageInterview, false},
		{"rejected from applied", StageApplied, StageRejected, true},
		{"rejected from screening", StageScreening, StageRejected, true},
		{"rejected from interview", StageInterview, StageRejected, true},
		{"withdrawn from any non-terminal stage", StageOffer, StageWithdrawn, true},
		{"terminal accepted has no outbound transitions", StageAccepted, StageScreening, false},
		{"terminal rejected has no outbound transitions", StageRejected, StageInterview, false},
		{"terminal rejected cannot be re-rejected", StageRejected, StageRejected, false},
		{"terminal withdrawn cannot un-withdraw", StageWithdrawn, StageApplied, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CanTransition(tc.from, tc.to)
			if got != tc.want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestStageValid(t *testing.T) {
	if !StageApplied.Valid() {
		t.Error("StageApplied should be valid")
	}
	if Stage("bogus").Valid() {
		t.Error("unknown stage should not be valid")
	}
}
