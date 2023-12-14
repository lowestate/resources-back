package repository

import (
	"ResourceExtraction/internal/app/ds"
	mClient "ResourceExtraction/internal/app/minio"
	"ResourceExtraction/internal/app/role"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"os"
	"slices"
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

func (r *Repository) GenerateHashString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

//---------------------------------------------------------------------------
//-------------------------------- RESOURCES --------------------------------

func (r *Repository) GetResourceByID(id uint) (*ds.Resources, error) {
	resource := &ds.Resources{}

	err := r.db.First(resource, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return resource, nil
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
		qry = qry.Where("status = ?", ds.ReqStatuses[1])
	} else {
		qry = qry.Where("status IN ?", ds.ReqStatuses)
	}

	err := qry.
		Preload("Client").Preload("Moderator"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) GetReportsByStatus(status string) ([]ds.ExtractionReports, error) {
	requests := []ds.ExtractionReports{}

	err := r.db.
		Preload("client_ref").Preload("moderator_ref"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Where("status = ?", status).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) GetCurrentReport(client_refer uuid.UUID) (*ds.ExtractionReports, error) {
	request := &ds.ExtractionReports{}
	err := r.db.Where("status = ?", "Черновик").First(request, "client_ref = ?", client_refer).Error
	//если реквеста нет => err = record not found
	if err != nil {
		//request = nil, err = not found
		return nil, err
	}
	//если реквест есть => request = record, err = nil
	return request, nil
}

func (r *Repository) CreateTransferRequest(requestBody ds.CreateReportBody, userUUID uuid.UUID) error {
	var orbit_ids []int
	var orbit_names []string
	for _, orbitName := range requestBody.Resources {
		orbit, err := r.GetResourceByName(orbitName)
		if err != nil {
			return err
		}
		orbit_ids = append(orbit_ids, int(orbit.ID))
		orbit_names = append(orbit_names, orbit.ResourceName)
	}

	request, err := r.GetCurrentReport(userUUID)
	if err != nil {
		log.Println(" --- NEW REQUEST --- ", userUUID)

		//назначение модератора
		moders := []ds.Users{}
		err = r.db.Where("role = ?", 1).Find(&moders).Error
		if err != nil {
			return err
		}
		n := rand.Int() % len(moders)
		moder_refer := moders[n].UUID
		log.Println("moder: ", moder_refer)

		//поля типа Users, связанные с передавыемыми значениями из функции
		client := ds.Users{UUID: userUUID}
		moder := ds.Users{UUID: moder_refer}

		request = &ds.ExtractionReports{
			ID:            uint(len([]ds.ExtractionReports{})),
			ClientRef:     userUUID,
			Client:        client,
			ModeratorRef:  moder_refer,
			Moderator:     moder,
			Status:        "Черновик",
			DateCreated:   time.Now(),
			DateProcessed: nil,
			DateFinished:  nil,
		}

		err := r.db.Create(request).Error
		if err != nil {
			return err
		}
	}

	err = r.SetRequestOrbits(int(request.ID), orbit_names)
	if err != nil {
		return err
	}

	//for _, orbit_id := range orbit_ids {
	//	transfer_to_orbit := ds.TransferToOrbit{}
	//	transfer_to_orbit.RequestRefer = request.ID
	//	transfer_to_orbit.OrbitRefer = uint(orbit_id)
	//	err = r.CreateTransferToOrbit(transfer_to_orbit)
	//
	//	if err != nil {
	//		return err
	//	}
	//}

	return nil

}

func (r *Repository) SetRequestOrbits(transferID int, orbits []string) error {
	var orbit_ids []int
	log.Println(transferID, " - ", orbits)
	for _, orbit_name := range orbits {
		orbit, err := r.GetResourceByName(orbit_name)
		log.Println("orbit: ", orbit)
		if err != nil {
			return err
		}

		for _, ele := range orbit_ids {
			if ele == int(orbit.ID) {
				log.Println("!!!")
				continue
			}
		}
		log.Println("BEFORE :", orbit_ids)
		orbit_ids = append(orbit_ids, int(orbit.ID))
		log.Println("AFTER :", orbit_ids)
	}

	var existing_links []ds.ManageReports
	err := r.db.Model(&ds.ManageReports{}).Where("report_ref = ?", transferID).Find(&existing_links).Error
	if err != nil {
		return err
	}
	log.Println("LINKS: ", existing_links)
	for _, link := range existing_links {
		orbitFound := false
		orbitIndex := -1
		for index, ele := range orbit_ids {
			if ele == int(link.ResourceRef) {
				orbitFound = true
				orbitIndex = index
				break
			}
		}
		log.Println("ORB F: ", orbitFound)
		if orbitFound {
			log.Println("APPEND: ")
			orbit_ids = append(orbit_ids[:orbitIndex], orbit_ids[orbitIndex+1:]...)
		} else {
			log.Println("DELETE: ")
			err := r.db.Model(&ds.ManageReports{}).Delete(&link).Error
			if err != nil {
				return err
			}
		}
	}

	for _, orbit_id := range orbit_ids {
		newLink := ds.ManageReports{
			ReportRef:   uint(transferID),
			ResourceRef: uint(orbit_id),
		}
		log.Println("NEW LINK", newLink.ResourceRef, " --- ", newLink.ReportRef)
		err := r.db.Model(&ds.ManageReports{}).Create(&newLink).Error
		if err != nil {
			return nil
		}
	}

	return nil
}

/*
func (r *Repository) AddReportToMM(resourceRef, reportRef uint) error {
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
*/

func (r *Repository) CreateTransferToOrbit(MM ds.ManageReports) error {
	return r.db.Create(&MM).Error
}

func (r *Repository) GetOrbitsFromTransfer(id int) ([]ds.Resources, error) {
	MM := []ds.ManageReports{}

	err := r.db.Model(&ds.ManageReports{}).Where("report_ref = ?", id).Find(&MM).Error
	if err != nil {
		return []ds.Resources{}, err
	}

	var orbits []ds.Resources
	for _, transfer_to_orbit := range MM {
		orbit, err := r.GetResourceByID(transfer_to_orbit.ResourceRef)
		if err != nil {
			return []ds.Resources{}, err
		}
		for _, ele := range orbits {
			if ele == *orbit {
				continue
			}
		}
		orbits = append(orbits, *orbit)
	}

	return orbits, nil

}

func (r *Repository) GetReportByID(id uint, userUUID uuid.UUID, userRole any) (*ds.ExtractionReports, error) {
	request := &ds.ExtractionReports{}
	qry := r.db

	if userRole == role.User {
		qry = qry.Where("client_ref = ?", userUUID)
	} else {
		qry = qry.Where("moderator_ref = ?", userUUID)
	}

	err := qry.Preload("Client").Preload("Moderator").First(request, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return request, nil
}

func (r *Repository) ChangeReportStatus(id uint, status string) error {
	if slices.Contains(ds.ReqStatuses[2:4], status) {
		err := r.db.Model(&ds.ExtractionReports{}).Where("id = ?", id).Update("date_finished", time.Now()).Error
		if err != nil {
			return err
		}
	}

	if status == ds.ReqStatuses[1] {
		err := r.db.Model(&ds.ExtractionReports{}).Where("id = ?", id).Update("date_processed", time.Now()).Error
		if err != nil {
			return err
		}
	}

	err := r.db.Model(&ds.ExtractionReports{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса: %w", err)
	}

	if status == ds.ReqStatuses[2] {
		err = r.DeleteAllResourcesFromMM(id)
	}

	return nil
}

func (r *Repository) DeleteReport(id uint) error {
	if r.db.Where("id = ?", id).First(&ds.ExtractionReports{}).Error != nil {

		return r.db.Where("id = ?", id).First(&ds.ExtractionReports{}).Error
	}
	return r.db.Model(&ds.ExtractionReports{}).Where("id = ?", id).Update("status", "Удалена").Error
}

func (r *Repository) DeleteOneResourceFromMM(report_id, resource_id uint) (error, error) {
	if r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error != nil ||
		r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error != nil {

		return r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error,
			r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error
	}
	return r.db.Where("report_ref = ?", report_id).Where("resource_ref = ?", resource_id).Delete(&ds.ManageReports{}).Error, nil
}

func (r *Repository) DeleteAllResourcesFromMM(report_id uint) error {
	if r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error != nil {
		return r.db.Where("report_ref = ?", report_id).First(&ds.ManageReports{}).Error
	}
	return r.db.Where("report_ref = ?", report_id).Delete(&ds.ManageReports{}).Error
}

//---------------------------------------------------------------------------
//---------------------------------- USERS ----------------------------------

func (r *Repository) Register(user *ds.Users) error {
	if user.UUID == uuid.Nil {
		user.UUID = uuid.New()
	}

	return r.db.Create(user).Error
}

func (r *Repository) GetUserByName(name string) (*ds.Users, error) {
	user := &ds.Users{}

	err := r.db.First(user, "username = ?", name).Error
	if err != nil {
		return nil, err
	}

	return user, nil
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
