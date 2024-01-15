package app

import (
	"fmt"
	"log"
	"net/http"

	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/repository"

	"github.com/gin-gonic/gin"
)

func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	a.r.LoadHTMLGlob("templates/*.html")
	a.r.Static("/css", "./css")

	a.r.GET("/home", a.loadHome)
	a.r.GET("/home/:title", a.loadPage)

	a.r.POST("/home/delete_resource/:resource_name", func(c *gin.Context) {
		resource_name := c.Param("resource_name")
		err := a.repo.ChangeAvailability(resource_name)
		if err != nil {
			c.Error(err)
			return
		}

		c.Redirect(http.StatusFound, "/home")
		fmt.Println("redirected")
	})

	a.r.Run(":8080")

	log.Println("Server is down")
}

type Application struct {
	repo repository.Repository
	r    *gin.Engine
}

func New() Application {
	app := Application{}

	repo, _ := repository.New(dsn.FromEnv())

	app.repo = *repo

	return app
}

func (a *Application) loadHome(c *gin.Context) {
	resourceName := c.DefaultQuery("search", "")
	fmt.Println(resourceName)

	if resourceName == "" {
		allRes, err := a.repo.GetAllResources()
		if err != nil {
			c.Error(err)
		}
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"materials": allRes,
		})
	} else {
		foundResources, err := a.repo.SearchResources(resourceName)

		if err != nil {
			c.Error(err)
			return
		}

		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"materials": a.repo.FilteredResources(foundResources),
			"Material":  resourceName,
		})
	}
}

func (a *Application) loadPage(c *gin.Context) {
	resource_name := c.Param("title")

	if resource_name == "favicon.ico" {
		return
	}

	resource, err := a.repo.GetResourceByName(resource_name)

	if err != nil {
		c.Error(err)
		return
	}

	c.HTML(http.StatusOK, "rp_resource.html", gin.H{
		"ResourceName":    resource.ResourceName,
		"Image":           resource.Image,
		"AmountAvailable": resource.AmountAvailable,
		"IsAvailable":     resource.IsAvailable,
		"Place":           resource.Place,
	})

}
