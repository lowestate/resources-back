package repository

import (
	"ResourceExtraction/internal/app/ds"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"strings"
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

func firstLetterToHigher(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToTitle(r[0])
	return string(r)
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

func (r *Repository) GetAllResources() ([]ds.Resources, error) {
	resources := []ds.Resources{}

	err := r.db.Find(&resources).Error

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

	return resource, nil
}

func (r *Repository) DeleteUser(username string) error {
	return r.db.Delete(&ds.Users{}, "username = ?", username).Error
}

func (r *Repository) CreateUser(user ds.Users) error {
	return r.db.Create(user).Error
}
