package lang_dto

const (
	StatusPlanned = "planned"
	DirectionNext = "next"
	DirectionPrev = "prev"
)

type Create struct {
	Name     string `json:"name" validate:"required,min=2,max=10"`
	IsActive bool   `json:"is_active"`
}

type Update Create

type Filter struct {
	Cursor    int64
	Limit     int
	Name      string
	IsActive  *bool
	Direction string
}
