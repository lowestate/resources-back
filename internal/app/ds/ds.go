package ds

import (
	"ResourceExtraction/internal/app/role"
	"github.com/google/uuid"
	"time"
)

/*
	Статусы заявок ('Status'):
	1. Черновик - на редактировании клиентом
	2. Удалена - удалена клиентом (не отправлена, отменена)
	3. На рассмотрении - отправлена клиентом, проходит проверку у модератора
	4. Оказана - одобрена модератором (завершена успешно)
	5. Отклонена - не одобрена модератором (завершена неуспешно)
*/

type UserUuid struct {
	UUID     uuid.UUID `gorm:"type:uuid"`
	Username string    `json:"name"`
	Role     role.Role `sql:"type:string;"`
	Password string
}

// ---------------------------------------------------------------//
type Users struct {
	ID       uint `gorm:"primaryKey;AUTO_INCREMENT"`
	Username string
	IsAdmin  bool
	Password string
}

type Resources struct {
	ID                uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ResourceName      string
	IsAvailable       bool
	Month             string
	MonthlyProduction float64
	Place             string
	Image             string `gorm:"column:image"`
}

type ExtractionReports struct {
	ID           uint `gorm:"primaryKey;AUTO_INCREMENT"`
	Status       string
	DateCreated  time.Time  `gorm:"type:timestamp"` // не может быть пустой = без *
	DateFormed   *time.Time `gorm:"type:timestamp"`
	DateFinished *time.Time `gorm:"type:timestamp"`
	ClientRef    int
	Client       Users `gorm:"foreignKey:ClientRef"`
	ModeratorRef int
	Moderator    Users `gorm:"foreignKey:ModeratorRef"`
}

type ManageReports struct {
	ID          uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ReportRef   int
	IdReport    ExtractionReports `gorm:"foreignKey:ReportRef"`
	ResourceRef int
	IdResource  Resources `gorm:"foreignKey:ResourceRef"`
}

// JSON PARSER
type AddResRequestBody struct {
	ResourceName string
	Place        string
	Image        string
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
