package ds

import (
	"gorm.io/datatypes"
)

// DB MIGRATION
type Statuses struct {
	ID     uint `gorm:"primaryKey"`
	Status string
}

type Users struct {
	ID       uint `gorm:"primaryKey"`
	Username string
	IsAdmin  bool
}

type Resources struct {
	ID                uint `gorm:"primaryKey"`
	ResourceName      string
	IsAvailable       bool
	Month             string
	MonthlyProduction float64
	Place             string
	Image             string `gorm:"type:bytea"`
}

type ExtractionReports struct {
	ID           uint `gorm:"primaryKey"`
	StatusRef    int
	Status       Statuses `gorm:"foreignKey:StatusRef"`
	DateCreated  datatypes.Date
	DateFormed   datatypes.Date
	DateFinished datatypes.Date
	Month        string
	Place        string
	ClientRef    int
	Client       Users `gorm:"foreignKey:ClientRef"`
	ModeratorRef int
	Moderator    Users `gorm:"foreignKey:ModeratorRef"`
}

type ManageReports struct {
	ReportRef   int
	IdReport    ExtractionReports `gorm:"foreignKey:ReportRef"`
	ResourceRef int
	IdResource  Resources `gorm:"foreignKey:ResourceRef"`
}

// JSON PARSER
type AddResRequestBody struct {
	ResourceName string
	Place        string
}

type EditResNameRequestBody struct {
	OldName string
	NewName string
}

// тут не нужно поле с названием ресурса потому что поступает запрос .../:resource_name/addmonthlyprod
type AddMonthlyProd struct {
	Place       string
	Month       string
	MonthlyProd float64
}

type AddReportRequestBody struct {
	ResourceNeeded string
	Place          string
}
