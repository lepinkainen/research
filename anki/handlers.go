package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func pathID(r *http.Request, name string) (int64, error) {
	s := r.PathValue(name)
	if s == "" {
		return 0, errors.New("missing id")
	}
	return strconv.ParseInt(s, 10, 64)
}

func handleListDecks(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		decks, err := listDecks(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, decks)
	}
}

func handleCreateDeck(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		deck, err := createDeck(db, body.Name, body.Description)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, deck)
	}
}

func handleDeleteDeck(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := deleteDeck(db, id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleListCards(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deckID, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deck id")
			return
		}
		cards, err := listCards(db, deckID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, cards)
	}
}

func handleCreateCard(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deckID, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deck id")
			return
		}
		var body struct {
			Front string `json:"front"`
			Back  string `json:"back"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Front == "" || body.Back == "" {
			writeError(w, http.StatusBadRequest, "front and back are required")
			return
		}
		card, err := createCard(db, deckID, body.Front, body.Back)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, card)
	}
}

func handleDeleteCard(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid id")
			return
		}
		if err := deleteCard(db, id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleStudy(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deckID, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deck id")
			return
		}
		card, err := nextDueCard(db, deckID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if card == nil {
			writeJSON(w, http.StatusOK, map[string]bool{"done": true})
			return
		}
		writeJSON(w, http.StatusOK, card)
	}
}

func handleReview(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cardID, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid card id")
			return
		}
		var body struct {
			Rating int `json:"rating"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Rating < 0 || body.Rating > 3 {
			writeError(w, http.StatusBadRequest, "rating must be 0-3")
			return
		}

		var card Card
		err = db.QueryRow(`
			SELECT id, deck_id, front, back, created_at, updated_at,
			       due_date, interval_days, ease_factor, reps, lapses, state
			FROM cards WHERE id = ?
		`, cardID).Scan(&card.ID, &card.DeckID, &card.Front, &card.Back, &card.CreatedAt, &card.UpdatedAt,
			&card.DueDate, &card.IntervalDays, &card.EaseFactor, &card.Reps, &card.Lapses, &card.State)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "card not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		updated := Review(card, body.Rating, time.Now())
		if err := updateCardAfterReview(db, updated); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := insertRevLog(db, cardID, body.Rating, updated.IntervalDays, updated.EaseFactor); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"next_due":      updated.DueDate,
			"interval_days": updated.IntervalDays,
			"ease_factor":   updated.EaseFactor,
			"state":         updated.State,
		})
	}
}

func handleDeckStats(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deckID, err := pathID(r, "id")
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid deck id")
			return
		}
		stats, err := deckStats(db, deckID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stats)
	}
}
