package ds

import (
	"time"
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
	Status       string
	DateCreated  time.Time  `gorm:"type:timestamp"` // не может быть пустой = без *
	DateFormed   *time.Time `gorm:"type:timestamp"`
	DateFinished *time.Time `gorm:"type:timestamp"`
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
	Month       string
	MonthlyProd float64
}

type AddReportRequestBody struct {
	ResourceNeeded string
	Place          string
}

type ChangeStatusRequestBody struct {
	Who        string
	New_status string
}
