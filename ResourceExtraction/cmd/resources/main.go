package main

import (
	"ResourceExtraction/internal/pkg/app"
	"fmt"
)

func main() {
	fmt.Println("app started")
	a := app.New()
	a.StartServer()
	fmt.Println("app terminated")
}
