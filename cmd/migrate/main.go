package main

import (
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	// Migrate the schema

	err = db.AutoMigrate(&ds.Resources{}, &ds.ManageReports{})
	if err != nil {
		panic("cant migrate db")
	}
}
