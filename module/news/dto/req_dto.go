package news_dto

import category_dto "test/module/category/dto"

const (
	DirectionNext = category_dto.DirectionNext
	DirectionPrev = category_dto.DirectionPrev
)

type Translation struct {
	LanguageID  int64  `json:"language_id" validate:"required,min=1,max=2147483647"`
	Title       string `json:"title" validate:"required,max=255"`
	Description string `json:"description" validate:"required"`
}

type Create struct {
	IsMain       bool          `json:"is_main"`
	IsActive     bool          `json:"is_active"`
	Photo        string        `json:"photo"`
	Video        string        `json:"video"`
	UserID       int64         `json:"user_id" validate:"required,min=1"`
	CategoryIDs  []int64       `json:"category_ids" validate:"required,min=1,dive,min=1,max=2147483647"`
	Translations []Translation `json:"translations" validate:"dive"`
}

type Update Create

type Filter struct {
	LanguageID int64
	CategoryID int64
	UserID     int64
	Title      string
	IsMain     *bool
	IsActive   *bool
	Cursor     int64
	Limit      int
	Direction  string
}
