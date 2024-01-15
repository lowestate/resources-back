package ds

import (
	"gorm.io/datatypes"
)

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
	ID              uint `gorm:"primaryKey"`
	ResourceName    string
	IsAvailable     bool
	AmountAvailable float32
	Place           string
	Image           string `gorm:"type:bytea"`
}

type ExtractionReports struct {
	ID             uint `gorm:"primaryKey"`
	StatusRef      int
	Status         Statuses `gorm:"foreignKey:StatusRef"`
	DateCreated    datatypes.Date
	DateFormed     datatypes.Date
	DateFinished   datatypes.Date
	ResourceNeeded string
	ClientRef      int
	Client         Users `gorm:"foreignKey:ClientRef"`
	ModeratorRef   int
	Moderator      Users `gorm:"foreignKey:ModeratorRef"`
}

type ManageReports struct {
	ReportRef   int
	IdReport    ExtractionReports `gorm:"foreignKey:ReportRef"`
	ResourceRef int
	IdResource  Resources `gorm:"foreignKey:ResourceRef"`
}
