package main

import (
	"ResourceExtraction/internal/pkg/app"
	"fmt"
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
// @BasePath /home

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	fmt.Println("app started")
	a := app.New()
	a.StartServer()
	fmt.Println("app terminated")
}
