package repository

import (
	"ResourceExtraction/internal/app/ds"
	mClient "ResourceExtraction/internal/app/minio"
	"ResourceExtraction/internal/app/role"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"os"
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
	words := strings.Fields(s)
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToTitle(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func generateUniqueObjectName() string {
	// Ваш код для генерации уникального имени объекта, например, использование UUID
	// Пример: можно использовать github.com/google/uuid
	return uuid.New().String()
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

func isUnique(slice []string) bool {
	seen := make(map[string]int)
	for _, v := range slice {
		seen[v]++
		if seen[v] > 1 {
			return false
		}
	}
	return true
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

func (r *Repository) AddResource(resName string, place, imagePath string) error {
	image_placeholder := "http://127.0.0.1:9000/pc-bucket/placeholder.jpg"
	if imagePath == "" {
		imagePath = image_placeholder
	}
	return r.db.Create(&ds.Resources{
		uint(len([]ds.Resources{})),
		resName,
		true,
		"",
		0,
		place,
		imagePath}).Error
}

func (r *Repository) GetAllResources(title string) ([]ds.Resources, error) {
	resources := []ds.Resources{}
	title = firstLetterToHigher(strings.ToLower(title))
	if title != "" {
		log.Println("searching:", title)
	}
	err := r.db.Where("resource_name LIKE ?", "%"+title+"%").Find(&resources).Error
	if len(resources) == 0 && err == nil {
		log.Println("not found by name. searched by place")
		err1 := r.db.Where("place LIKE ?", "%"+title+"%").Find(&resources).Error
		if err1 != nil {
			return nil, err1
		}
	}

	resources = r.UniqueResources(resources)
	return resources, nil
}

// получение уникальных ресурсов для главной страницы
func (r *Repository) UniqueResources(allRes []ds.Resources) []ds.Resources {
	var newResources = []ds.Resources{}
	fmt.Println("before: ", len(allRes))

	for i := range allRes {
		var t = true
		for j := range newResources {
			if allRes[i].ResourceName == newResources[j].ResourceName {
				t = false
			}
		}
		if t {
			newResources = append(newResources, allRes[i])
		}
	}

	fmt.Println("after: ", len(newResources))
	return newResources
}

func (r *Repository) GetResourcesByPlace(place string) ([]ds.Resources, error) {
	resources := []ds.Resources{}

	err := r.db.Where("place = ?", place).Find(&resources).Error
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

func (r *Repository) GetResourcesByName(name string) ([]ds.Resources, error) {
	resources := []ds.Resources{}

	err := r.db.Where("resource_name = ?", name).Find(&resources).Error
	if err != nil {
		return nil, err
	}
	return resources, nil
}

func (r *Repository) EditResource(resource_name string, editingResource ds.Resources) error {
	originalResource, err := r.GetResourceByName(resource_name)
	if err != nil {
		return err
	}

	log.Println("OLD IMAGE: ", originalResource.Image)
	log.Println("NEW IMAGE: ", editingResource.Image)

	if editingResource.Image != originalResource.Image && editingResource.Image != "" {
		log.Println("REPLACING IMAGE")
		err := r.deleteImageFromMinio(originalResource.Image)
		if err != nil {
			return err
		}
		imageURL, err := r.uploadImageToMinio(editingResource.Image)
		if err != nil {
			return err
		}

		editingResource.Image = imageURL

		log.Println("IMAGE REPLACED")
	}

	return r.db.Model(&ds.Resources{}).Where("resource_name = ?", resource_name).Updates(editingResource).Error
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
	var editingResource ds.Resources
	resource, err := r.GetResourceByName(resName)
	// если такой ресурс уже есть, но без сведения о добыче хотя бы за один месяц - изменяем существующую запись
	if resource.MonthlyProduction == 0 && resource.Month == "" && err == nil {
		editingResource.Month = month
		editingResource.MonthlyProduction = monthlyProd
		return r.db.Model(&ds.Resources{}).Where("resource_name = ?", resName).Updates(editingResource).Error
	}
	// если же записи о месячной добыче у этого ресурса нет - создаем новую запись
	return r.db.Create(&ds.Resources{
		uint(len([]ds.Resources{})),
		resName,
		true,
		month,
		monthlyProd,
		place,
		"http://127.0.0.1:9000/pc-bucket/placeholder.jpg"}).Error
}

// ---------------------------------------------------------------------------
// --------------------------------- REPORTS ---------------------------------
func (r *Repository) GetAllRequests(userRole any, dateStart, dateFin string) ([]ds.ExtractionReports, error) {

	requests := []ds.ExtractionReports{}
	qry := r.db

	if dateStart != "" && dateFin != "" {
		qry = qry.Where("date_processed BETWEEN ? AND ?", dateStart, dateFin)
	} else if dateStart != "" {
		qry = qry.Where("date_processed >= ?", dateStart)
	} else if dateFin != "" {
		qry = qry.Where("date_processed <= ?", dateFin)
	}

	if userRole == role.Admin {
		qry = qry.Where("status = ?", ds.ReqStatuses[4])
	} else {
		qry = qry.Where("status IN ?", ds.ReqStatuses[:2])
	}

	err := qry.
		Preload("Client").Preload("Moder"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) GetCurrentReport(client_refer int) (*ds.ExtractionReports, error) {
	request := &ds.ExtractionReports{}
	err := r.db.Where("status = ?", "черновик").First(request, "client_ref = ?", client_refer).Error
	//если реквеста нет => err = record not found
	if err != nil {
		//request = nil, err = not found
		return nil, err
	}
	//если реквест есть => request = record, err = nil
	return request, nil
}

func (r *Repository) CreateNewReport(client_refer int, resource_name string) (*ds.ExtractionReports, error) {
	//проверка есть ли открытая заявка у клиента
	request, err := r.GetCurrentReport(client_refer)
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

		NewTransferRequest := &ds.ExtractionReports{
			ID:           uint(len([]ds.ExtractionReports{})),
			ClientRef:    client_refer,
			Client:       client,
			ModeratorRef: int(moder_refer),
			Moderator:    moder,
			Status:       "черновик",
			DateCreated:  time.Now(),
			DateFormed:   nil,
			DateFinished: nil,
		}
		log.Println("!!! NEW RECORD ADDED")
		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
	}
	return request, nil
}

func (r *Repository) AddReport(resourceRef, reportRef int) error {
	resource := ds.Resources{ID: uint(resourceRef)}
	report := ds.ExtractionReports{ID: uint(reportRef)}

	NewMtM := &ds.ManageReports{
		ID:          uint(len([]ds.ManageReports{})),
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

func (r *Repository) DeleteReport(id uint) (error, error) {
	return r.db.Delete(&ds.ManageReports{}, "report_ref = ?", id).Error,
		r.db.Model(&ds.ExtractionReports{}).Where(
			"id", id).Update(
			"status", "удален").Error
}

//---------------------------------------------------------------------------
//---------------------------------- USERS ----------------------------------

func (r *Repository) Register(user *ds.UserUuid) error {
	if user.UUID == uuid.Nil {
		user.UUID = uuid.New()
	}

	return r.db.Create(user).Error
}

func (r *Repository) GetUserByLogin(login string) (*ds.UserUuid, error) {
	user := &ds.UserUuid{}

	err := r.db.First(user, "username = ?", login).Error

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *Repository) DeleteUser(username string) error {
	return r.db.Delete(&ds.Users{}, "username = ?", username).Error
}

func (r *Repository) CreateUser(user ds.Users) error {
	return r.db.Create(user).Error
}

//---------------------------------------------------------------------------
//---------------------------------- MINIO ----------------------------------

func (r *Repository) uploadImageToMinio(imagePath string) (string, error) {
	// Получаем клиента Minio из настроек
	minioClient := mClient.NewMinioClient()

	// Загрузка изображения в Minio
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Генерация уникального имени объекта в Minio (например, используя UUID)
	objectName := generateUniqueObjectName() + ".jpg"

	_, err = minioClient.PutObject(context.Background(), "pc-bucket", objectName, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "", err
	}

	// Возврат URL изображения в Minio
	return fmt.Sprintf("http://%s/%s/%s", minioClient.EndpointURL().Host, "pc-bucket", objectName), nil
}

func (r *Repository) deleteImageFromMinio(imageURL string) error {
	minioClient := mClient.NewMinioClient()

	objectName := extractObjectNameFromURL(imageURL)

	return minioClient.RemoveObject(context.Background(), "pc-bucket", objectName, minio.RemoveObjectOptions{})
}

func extractObjectNameFromURL(imageURL string) string {
	parts := strings.Split(imageURL, "/")
	log.Println("\n\nIMG:   ", parts[len(parts)-1])
	return parts[len(parts)-1]
}
