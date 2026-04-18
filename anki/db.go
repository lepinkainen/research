package main

import (
	"database/sql"
	"time"
)

const schema = `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS decks (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL UNIQUE,
    description TEXT    NOT NULL DEFAULT '',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS cards (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deck_id       INTEGER NOT NULL REFERENCES decks(id) ON DELETE CASCADE,
    front         TEXT    NOT NULL,
    back          TEXT    NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    due_date      DATETIME NOT NULL DEFAULT (datetime('now')),
    interval_days REAL    NOT NULL DEFAULT 0,
    ease_factor   REAL    NOT NULL DEFAULT 2.5,
    reps          INTEGER NOT NULL DEFAULT 0,
    lapses        INTEGER NOT NULL DEFAULT 0,
    state         TEXT    NOT NULL DEFAULT 'new'
);

CREATE INDEX IF NOT EXISTS idx_cards_deck_due ON cards(deck_id, due_date);

CREATE TABLE IF NOT EXISTS revlog (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id       INTEGER NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    rating        INTEGER NOT NULL,
    interval_days REAL    NOT NULL,
    ease_factor   REAL    NOT NULL,
    reviewed_at   DATETIME NOT NULL DEFAULT (datetime('now'))
);
`

type Deck struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	TotalCards  int       `json:"total_cards"`
	DueCards    int       `json:"due_cards"`
}

type Card struct {
	ID           int64     `json:"id"`
	DeckID       int64     `json:"deck_id"`
	Front        string    `json:"front"`
	Back         string    `json:"back"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DueDate      time.Time `json:"due_date"`
	IntervalDays float64   `json:"interval_days"`
	EaseFactor   float64   `json:"ease_factor"`
	Reps         int       `json:"reps"`
	Lapses       int       `json:"lapses"`
	State        string    `json:"state"`
}

type RevLog struct {
	ID           int64     `json:"id"`
	CardID       int64     `json:"card_id"`
	Rating       int       `json:"rating"`
	IntervalDays float64   `json:"interval_days"`
	EaseFactor   float64   `json:"ease_factor"`
	ReviewedAt   time.Time `json:"reviewed_at"`
}

type DeckStats struct {
	Total    int `json:"total"`
	New      int `json:"new"`
	Learning int `json:"learning"`
	Review   int `json:"review"`
	DueToday int `json:"due_today"`
	Mature   int `json:"mature"`
}

func initDB(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

func listDecks(db *sql.DB) ([]Deck, error) {
	rows, err := db.Query(`
		SELECT d.id, d.name, d.description, d.created_at,
		       COUNT(c.id) AS total_cards,
		       SUM(CASE WHEN c.due_date <= datetime('now') THEN 1 ELSE 0 END) AS due_cards
		FROM decks d
		LEFT JOIN cards c ON c.deck_id = d.id
		GROUP BY d.id
		ORDER BY d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decks []Deck
	for rows.Next() {
		var d Deck
		var dueCards sql.NullInt64
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.CreatedAt, &d.TotalCards, &dueCards); err != nil {
			return nil, err
		}
		d.DueCards = int(dueCards.Int64)
		decks = append(decks, d)
	}
	if decks == nil {
		decks = []Deck{}
	}
	return decks, rows.Err()
}

func createDeck(db *sql.DB, name, description string) (Deck, error) {
	res, err := db.Exec(`INSERT INTO decks (name, description) VALUES (?, ?)`, name, description)
	if err != nil {
		return Deck{}, err
	}
	id, _ := res.LastInsertId()
	return getDeck(db, id)
}

func getDeck(db *sql.DB, id int64) (Deck, error) {
	var d Deck
	err := db.QueryRow(`SELECT id, name, description, created_at FROM decks WHERE id = ?`, id).
		Scan(&d.ID, &d.Name, &d.Description, &d.CreatedAt)
	return d, err
}

func deleteDeck(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM decks WHERE id = ?`, id)
	return err
}

func listCards(db *sql.DB, deckID int64) ([]Card, error) {
	rows, err := db.Query(`
		SELECT id, deck_id, front, back, created_at, updated_at,
		       due_date, interval_days, ease_factor, reps, lapses, state
		FROM cards WHERE deck_id = ? ORDER BY created_at DESC
	`, deckID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.DeckID, &c.Front, &c.Back, &c.CreatedAt, &c.UpdatedAt,
			&c.DueDate, &c.IntervalDays, &c.EaseFactor, &c.Reps, &c.Lapses, &c.State); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []Card{}
	}
	return cards, rows.Err()
}

func createCard(db *sql.DB, deckID int64, front, back string) (Card, error) {
	res, err := db.Exec(`INSERT INTO cards (deck_id, front, back) VALUES (?, ?, ?)`, deckID, front, back)
	if err != nil {
		return Card{}, err
	}
	id, _ := res.LastInsertId()
	var c Card
	err = db.QueryRow(`
		SELECT id, deck_id, front, back, created_at, updated_at,
		       due_date, interval_days, ease_factor, reps, lapses, state
		FROM cards WHERE id = ?
	`, id).Scan(&c.ID, &c.DeckID, &c.Front, &c.Back, &c.CreatedAt, &c.UpdatedAt,
		&c.DueDate, &c.IntervalDays, &c.EaseFactor, &c.Reps, &c.Lapses, &c.State)
	return c, err
}

func deleteCard(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM cards WHERE id = ?`, id)
	return err
}

// nextDueCard returns the next card to study, prioritising learning > review > new.
func nextDueCard(db *sql.DB, deckID int64) (*Card, error) {
	queries := []string{
		`SELECT id, deck_id, front, back, created_at, updated_at,
		        due_date, interval_days, ease_factor, reps, lapses, state
		 FROM cards WHERE deck_id = ? AND state = 'learning' AND due_date <= datetime('now')
		 ORDER BY due_date LIMIT 1`,
		`SELECT id, deck_id, front, back, created_at, updated_at,
		        due_date, interval_days, ease_factor, reps, lapses, state
		 FROM cards WHERE deck_id = ? AND state = 'review' AND due_date <= datetime('now')
		 ORDER BY due_date LIMIT 1`,
		`SELECT id, deck_id, front, back, created_at, updated_at,
		        due_date, interval_days, ease_factor, reps, lapses, state
		 FROM cards WHERE deck_id = ? AND state = 'new'
		 ORDER BY id LIMIT 1`,
	}
	for _, q := range queries {
		var c Card
		err := db.QueryRow(q, deckID).Scan(&c.ID, &c.DeckID, &c.Front, &c.Back, &c.CreatedAt, &c.UpdatedAt,
			&c.DueDate, &c.IntervalDays, &c.EaseFactor, &c.Reps, &c.Lapses, &c.State)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		return &c, nil
	}
	return nil, nil
}

func updateCardAfterReview(db *sql.DB, c Card) error {
	_, err := db.Exec(`
		UPDATE cards SET
			due_date = ?, interval_days = ?, ease_factor = ?,
			reps = ?, lapses = ?, state = ?, updated_at = datetime('now')
		WHERE id = ?
	`, c.DueDate, c.IntervalDays, c.EaseFactor, c.Reps, c.Lapses, c.State, c.ID)
	return err
}

func insertRevLog(db *sql.DB, cardID int64, rating int, intervalDays, easeFactor float64) error {
	_, err := db.Exec(`
		INSERT INTO revlog (card_id, rating, interval_days, ease_factor)
		VALUES (?, ?, ?, ?)
	`, cardID, rating, intervalDays, easeFactor)
	return err
}

func deckStats(db *sql.DB, deckID int64) (DeckStats, error) {
	var s DeckStats
	err := db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN state = 'new' THEN 1 ELSE 0 END),
			SUM(CASE WHEN state = 'learning' THEN 1 ELSE 0 END),
			SUM(CASE WHEN state = 'review' THEN 1 ELSE 0 END),
			SUM(CASE WHEN due_date <= datetime('now') THEN 1 ELSE 0 END),
			SUM(CASE WHEN state = 'review' AND interval_days >= 21 THEN 1 ELSE 0 END)
		FROM cards WHERE deck_id = ?
	`, deckID).Scan(&s.Total, &s.New, &s.Learning, &s.Review, &s.DueToday, &s.Mature)
	return s, err
}
