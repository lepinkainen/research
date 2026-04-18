package main

import (
	"math"
	"testing"
	"time"
)

var baseTime = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

func newCard(state string, reps int, interval, ease float64, lapses int) Card {
	return Card{
		State:        state,
		Reps:         reps,
		IntervalDays: interval,
		EaseFactor:   ease,
		Lapses:       lapses,
	}
}

func approx(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

func TestReview(t *testing.T) {
	tests := []struct {
		name          string
		card          Card
		rating        int
		wantState     string
		wantInterval  float64
		wantEase      float64
		wantReps      int
		wantLapses    int
	}{
		{
			name:         "new card Good",
			card:         newCard("new", 0, 0, 2.5, 0),
			rating:       2,
			wantState:    "learning",
			wantInterval: 1,
			wantEase:     2.5,
			wantReps:     1,
			wantLapses:   0,
		},
		{
			name:         "new card Again",
			card:         newCard("new", 0, 0, 2.5, 0),
			rating:       0,
			wantState:    "learning",
			wantInterval: 1,
			wantEase:     2.5,
			wantReps:     0,
			wantLapses:   1,
		},
		{
			name:         "learning second step Good → graduate",
			card:         newCard("learning", 1, 1, 2.5, 0),
			rating:       2,
			wantState:    "review",
			wantInterval: 6,
			wantEase:     2.5,
			wantReps:     2,
			wantLapses:   0,
		},
		{
			name:         "learning Again stays learning",
			card:         newCard("learning", 1, 1, 2.5, 0),
			rating:       0,
			wantState:    "learning",
			wantInterval: 1,
			wantEase:     2.3,
			wantReps:     1,
			wantLapses:   1,
		},
		{
			name:         "review Good",
			card:         newCard("review", 3, 6, 2.5, 0),
			rating:       2,
			wantState:    "review",
			wantInterval: 15,
			wantEase:     2.5,
			wantReps:     4,
			wantLapses:   0,
		},
		{
			name:         "review Hard",
			card:         newCard("review", 3, 6, 2.5, 0),
			rating:       1,
			wantState:    "review",
			wantInterval: 7.2,
			wantEase:     2.35,
			wantReps:     4,
			wantLapses:   0,
		},
		{
			name:         "review Easy",
			card:         newCard("review", 3, 6, 2.5, 0),
			rating:       3,
			wantState:    "review",
			wantInterval: 20.67, // 6 * 2.65 * 1.3
			wantEase:     2.65,
			wantReps:     4,
			wantLapses:   0,
		},
		{
			name:         "review Again → learning",
			card:         newCard("review", 3, 6, 2.5, 0),
			rating:       0,
			wantState:    "learning",
			wantInterval: 1,
			wantEase:     2.3,
			wantReps:     3,
			wantLapses:   1,
		},
		{
			name:         "ease floor on Again",
			card:         newCard("review", 3, 6, 1.3, 2),
			rating:       0,
			wantState:    "learning",
			wantInterval: 1,
			wantEase:     1.3,
			wantReps:     3,
			wantLapses:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Review(tt.card, tt.rating, baseTime)
			if got.State != tt.wantState {
				t.Errorf("State: got %q, want %q", got.State, tt.wantState)
			}
			if !approx(got.IntervalDays, tt.wantInterval) {
				t.Errorf("IntervalDays: got %.4f, want %.4f", got.IntervalDays, tt.wantInterval)
			}
			if !approx(got.EaseFactor, tt.wantEase) {
				t.Errorf("EaseFactor: got %.4f, want %.4f", got.EaseFactor, tt.wantEase)
			}
			if got.Reps != tt.wantReps {
				t.Errorf("Reps: got %d, want %d", got.Reps, tt.wantReps)
			}
			if got.Lapses != tt.wantLapses {
				t.Errorf("Lapses: got %d, want %d", got.Lapses, tt.wantLapses)
			}
			expectedDue := baseTime.Add(time.Duration(float64(time.Hour) * 24 * tt.wantInterval))
			diff := got.DueDate.Sub(expectedDue)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Errorf("DueDate: got %v, want ~%v (diff %v)", got.DueDate, expectedDue, diff)
			}
		})
	}
}
