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
var ReqStatuses = []string{
	"Черновик",
	"Сформирована",
	"Удалена",
	"Отклонена",
	"Оказана",
}

// ---------------------------------------------------------------//
type Users struct {
	UUID     uuid.UUID `gorm:"type:uuid"`
	Username string    `json:"Username"`
	Role     role.Role `sql:"type:string;"`
	Password string
}

type Resources struct {
	ID           uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ResourceName string
	IsAvailable  bool
	Density      float64
	IsToxic      bool
	Demand       int8
	Image        string `gorm:"column:image"`
	Description  string
}

type ExtractionReports struct {
	ID            uint `gorm:"primaryKey;AUTO_INCREMENT"`
	Status        string
	DateCreated   time.Time  `gorm:"type:timestamp"` // не может быть пустой = без *
	DateProcessed *time.Time `gorm:"type:timestamp"`
	DateFinished  *time.Time `gorm:"type:timestamp"`
	ClientRef     uuid.UUID
	Client        Users `gorm:"foreignKey:ClientRef;references:UUID"`
	ModeratorRef  uuid.UUID
	Moderator     Users `gorm:"foreignKey:ModeratorRef;references:UUID"`
	Month         string
	Place         string
}

type ManageReports struct {
	ID          uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ReportRef   uint
	IdReport    ExtractionReports `gorm:"foreignKey:ReportRef"`
	ResourceRef uint
	IdResource  Resources `gorm:"foreignKey:ResourceRef"`
	Plan        float64
	Fact        float64
}

// JSON PARSER
type AddResRequestBody struct {
	ResourceName string
	Density      float64
	IsToxic      bool
	Demand       int8
	Image        string
	Desc         string
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
	ReportID uint   `json:"report_id"`
	Status   string `json:"status"`
}

type CreateReportBody struct {
	Resource string
}

type SetReportResourcesBody struct {
	ReportID  int      `json:"report_id"`
	Resources []string `json:"resources"`
}

type AddPlanToMM struct {
	ReportRef   uint    `json:"report_ref"`
	ResourceRef string  `json:"resource_ref"`
	Plan        float64 `json:"plan"`
}

type AddDataToReport struct {
	Place string `json:"place"`
	Month string `json:"month"`
}

type AsyncBody struct {
	ReportID   int `json:"report_ref"`
	ResourceID int `json:"resource_ref"`
	Fact       int `json:"fact"`
}

type DeletSingleFromMMBody struct {
	ResourceName string `json:"resource"`
	RequestID    string `json:"req"`
}
