package category_dto

const (
	DirectionNext = "next"
	DirectionPrev = "prev"
)

type Translation struct {
	LanguageID int64  `json:"language_id" validate:"required,min=1,max=2147483647"`
	Name       string `json:"name" validate:"required,max=255"`
}

type Create struct {
	Default      string        `json:"default" validate:"required,max=255"`
	IsActive     bool          `json:"is_active"`
	SortOrder    int           `json:"sort_order" validate:"min=0,max=2147483647"`
	Translations []Translation `json:"translations" validate:"dive"`
}

type Update Create

type Filter struct {
	LanguageID int64
	Cursor     int64
	Limit      int
	Name       string
	IsActive   *bool
	Direction  string
}
