package repository

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"awesomeProject1/internal/app/ds"
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

func (r *Repository) GetMaterialByID(id int) (*ds.Material, error) {
	material := &ds.Material{}

	err := r.db.First(material, "id = ?", "1").Error // find product with code D42
	if err != nil {
		return nil, err
	}

	return material, nil
}

func (r *Repository) CreateMaterial(product ds.Material) error {
	return r.db.Create(product).Error
}
