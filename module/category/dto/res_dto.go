package category_dto

import "time"

type MissingLanguage struct {
	LanguageID int64  `json:"language_id"`
	Name       string `json:"name"`
}

type MissingTranslationsResponse struct {
	Error            string            `json:"error"`
	MissingLanguages []MissingLanguage `json:"missing_languages"`
}

type Response struct {
	ID           int64         `json:"id"`
	Default      string        `json:"default"`
	IsActive     bool          `json:"is_active"`
	SortOrder    int           `json:"sort_order"`
	Translations []Translation `json:"translations"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
}

type ListItem struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListResponse struct {
	Data       []ListItem `json:"data"`
	NextCursor *int64     `json:"next_cursor"`
	PrevCursor *int64     `json:"prev_cursor"`
	HasNext    bool       `json:"has_next"`
	HasPrev    bool       `json:"has_prev"`
}
