package main

import (
	"awesomeProject1/internal/api"
	"fmt"
)

func main() {
	fmt.Println("app started")
	api.StartServer()
	fmt.Println("app terminated")
}
