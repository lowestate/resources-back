package main

import (
	"ResourceExtraction/internal/pkg/app"
	"context"
	"log"
)

// @title Resource Extraction
// @version 1.0
// @description API Server for Resource Extraction WebApp

// @contact.name API Support
// @contact.url https://vk.com/bmstu_schedule
// @contact.email bitop@spatecon.ru

// @license.name AS IS (NO WARRANTY)

// @host localhost:8080
// @schemes https http
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	log.Println("Application start!")

	a, err := app.New(context.Background())
	if err != nil {
		log.Println(err)

		return
	}

	a.StartServer()

	log.Println("Application terminated!")
}
