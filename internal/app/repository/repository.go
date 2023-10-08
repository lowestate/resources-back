package repository

import (
	"ResourceExtraction/internal/app/ds"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"strings"
	"time"
	"unicode"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &Repository{
		db: db,
	}, nil
}

func firstLetterToHigher(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToTitle(r[0])
	return string(r)
}

//---------------------------------------------------------------------------
//-------------------------------- RESOURCES --------------------------------

func (r *Repository) GetResourceByID(id int) (*ds.Resources, error) {
	resource := &ds.Resources{}

	err := r.db.First(resource, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (r *Repository) SearchResources(resName string) ([]ds.Resources, error) {
	allresources, _ := r.GetAllResources()

	resName = strings.ToLower(resName)
	resName = firstLetterToHigher(resName)
	resName = "%" + resName + "%"

	err := r.db.Where("resource_name LIKE ?", resName).Find(&allresources).Error
	if err != nil {
		return nil, err
	}

	return allresources, nil
}

func (r *Repository) DeleteResource(resName string) error {
	return r.db.Delete(&ds.Resources{}, "resource_name = ?", resName).Error
}

func (r *Repository) ChangeAvailability(resName string) error {
	query := "UPDATE resources SET is_available = NOT is_available WHERE resource_name = $1"

	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	strings.ToLower(resName)
	firstLetterToHigher(resName)
	_, err = sqlDB.Exec(query, resName)

	return err
}

func (r *Repository) AddResource(resName string, place string) error {
	return r.db.Create(&ds.Resources{
		uint(len([]ds.Resources{})),
		resName,
		true,
		"",
		0,
		place,
		""}).Error
}

func (r *Repository) GetAllResources() ([]ds.Resources, error) {
	resources := []ds.Resources{}

	err := r.db.Order("id").Find(&resources).Error

	if err != nil {
		return nil, err
	}

	return resources, nil
}

func (r *Repository) FilteredResources(resources []ds.Resources) []ds.Resources {
	var newResources = []ds.Resources{}

	for i := range resources {
		newResources = append(newResources, resources[i])
	}

	return newResources
}

func (r *Repository) GetResourceByName(name string) (*ds.Resources, error) {
	resource := &ds.Resources{}

	err := r.db.First(resource, "resource_name = ?", name).Error
	if err != nil {
		return nil, err
	}
	log.Println("!!!:  ", resource.ResourceName)
	return resource, nil
}

func (r *Repository) EditResourceName(resName, newName string) error {
	return r.db.Model(&ds.Resources{}).Where(
		"resource_name", resName).Update(
		"resource_name", newName).Error
}

func (r *Repository) AddMonthlyProd(resName, place, month string, monthlyProd float64) error {
	/*
		resources, _ := r.GetAllResources()
		for x := range resources {
			if resources[x].ResourceName == resName && resources[x].Place == place {
				return r.db.Model(&ds.Resources{}).Where(
					"resource_name = ?", resName).Where("place = ?", place).Update(
					"monthly_production", monthlyProd).Update("month", month).Error
			}
		}
	*/
	return r.db.Create(&ds.Resources{
		uint(len([]ds.Resources{})),
		resName,
		true,
		month,
		monthlyProd,
		place,
		""}).Error
}

// ---------------------------------------------------------------------------
// --------------------------------- REPORTS ---------------------------------
func (r *Repository) GetCurrentRequest(client_refer int) (*ds.ExtractionReports, error) {
	request := &ds.ExtractionReports{}
	err := r.db.Where("status = ?", "Opened").First(request, "client_ref = ?", client_refer).Error
	//если реквеста нет => err = record not found
	if err != nil {
		//request = nil, err = not found
		return nil, err
	}
	//если реквест есть => request = record, err = nil
	return request, nil
}

func (r *Repository) CreateTransferRequest(client_refer int, resource_name string) (*ds.ExtractionReports, error) {
	//проверка есть ли открытая заявка у клиента
	request, err := r.GetCurrentRequest(client_refer)
	if err != nil {
		log.Println("NO OPENED REQUESTS => CREATING NEW ONE")

		//назначение модератора
		users := []ds.Users{}
		err = r.db.Where("is_admin = ?", true).Find(&users).Error
		if err != nil {
			return nil, err
		}
		n := rand.Int() % len(users)
		moder_refer := users[n].ID

		//поля типа Users, связанные с передавыемыми значениями из функции
		client := ds.Users{ID: uint(client_refer)}
		moder := ds.Users{ID: moder_refer}

		resource, _ := r.GetResourceByName(resource_name)

		NewTransferRequest := &ds.ExtractionReports{
			ID:           uint(len([]ds.ExtractionReports{})),
			ClientRef:    client_refer,
			Client:       client,
			ModeratorRef: int(moder_refer),
			Moderator:    moder,
			Status:       "Opened",
			DateCreated:  time.Now(),
			DateFormed:   nil,
			DateFinished: nil,
			Place:        resource.Place,
		}
		log.Println("!!! NEW RECORD ADDED")
		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
	}
	return request, nil
}

func (r *Repository) AddTransferToOrbits(reportRef, resourceRef int) error {
	resource := ds.Resources{ID: uint(reportRef)}
	report := ds.ExtractionReports{ID: uint(resourceRef)}

	NewMtM := &ds.ManageReports{
		ReportRef:   reportRef,
		IdReport:    report,
		ResourceRef: resourceRef,
		IdResource:  resource,
	}

	return r.db.Create(NewMtM).Error
}

func (r *Repository) GetReportByID(id uint) (*ds.ExtractionReports, error) {
	reports := &ds.ExtractionReports{}

	err := r.db.First(reports, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return reports, nil
}

func (r *Repository) EditStatus(id uint, status string) error {
	return r.db.Model(&ds.ExtractionReports{}).Where(
		"id", id).Update(
		"status", status).Error
}

//---------------------------------------------------------------------------
//---------------------------------- USERS ----------------------------------

func (r *Repository) DeleteUser(username string) error {
	return r.db.Delete(&ds.Users{}, "username = ?", username).Error
}

func (r *Repository) CreateUser(user ds.Users) error {
	return r.db.Create(user).Error
}
