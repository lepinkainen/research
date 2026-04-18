package main

import (
	"math"
	"time"
)

const (
	minEaseFactor     = 1.3
	defaultEaseFactor = 2.5
)

// Review applies the SM-2 spaced repetition algorithm and returns the updated card.
// Rating: 0=Again, 1=Hard, 2=Good, 3=Easy
func Review(card Card, rating int, now time.Time) Card {
	switch card.State {
	case "new":
		if rating == 0 {
			card.State = "learning"
			card.IntervalDays = 1
			card.Lapses++
		} else {
			card.State = "learning"
			card.IntervalDays = 1
			card.Reps++
		}

	case "learning":
		if rating == 0 {
			card.IntervalDays = 1
			card.EaseFactor = math.Max(minEaseFactor, card.EaseFactor-0.2)
			card.Lapses++
		} else if card.Reps <= 1 {
			// Graduate after second successful step
			card.IntervalDays = 6
			card.State = "review"
			card.Reps++
		} else {
			card.IntervalDays = 6
			card.State = "review"
			card.Reps++
		}

	case "review":
		switch rating {
		case 0: // Again
			card.EaseFactor = math.Max(minEaseFactor, card.EaseFactor-0.2)
			card.IntervalDays = 1
			card.State = "learning"
			card.Lapses++
		case 1: // Hard
			card.EaseFactor = math.Max(minEaseFactor, card.EaseFactor-0.15)
			card.IntervalDays = math.Max(1, card.IntervalDays*1.2)
			card.Reps++
		case 2: // Good
			card.IntervalDays = math.Max(1, card.IntervalDays*card.EaseFactor)
			card.Reps++
		case 3: // Easy
			card.EaseFactor += 0.15
			card.IntervalDays = math.Max(1, card.IntervalDays*card.EaseFactor*1.3)
			card.Reps++
		}
	}

	card.DueDate = now.Add(time.Duration(float64(time.Hour) * 24 * card.IntervalDays))
	return card
}
