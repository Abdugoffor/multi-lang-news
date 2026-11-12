package news_dto

import "time"

type MissingLanguage struct {
	LanguageID int64  `json:"language_id"`
	Name       string `json:"name"`
}

type MissingTranslationsResponse struct {
	Error            string            `json:"error"`
	MissingLanguages []MissingLanguage `json:"missing_languages"`
}

type TranslationResponse struct {
	NewsID      int64     `json:"news_id"`
	LanguageID  int64     `json:"language_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Response struct {
	ID           int64                 `json:"id"`
	IsMain       bool                  `json:"is_main"`
	IsActive     bool                  `json:"is_active"`
	ViewCount    int64                 `json:"view_count"`
	Photo        string                `json:"photo"`
	Video        string                `json:"video"`
	UserID       int64                 `json:"user_id"`
	CategoryIDs  []int64               `json:"category_ids"`
	Translations []TranslationResponse `json:"translations"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type ListItem struct {
	ID          int64     `json:"id"`
	IsMain      bool      `json:"is_main"`
	IsActive    bool      `json:"is_active"`
	ViewCount   int64     `json:"view_count"`
	Photo       string    `json:"photo"`
	Video       string    `json:"video"`
	UserID      int64     `json:"user_id"`
	CategoryIDs []int64   `json:"category_ids"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ListResponse struct {
	Data       []ListItem `json:"data"`
	NextCursor *int64     `json:"next_cursor"`
	PrevCursor *int64     `json:"prev_cursor"`
	HasNext    bool       `json:"has_next"`
	HasPrev    bool       `json:"has_prev"`
}
