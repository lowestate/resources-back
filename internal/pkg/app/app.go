package app

import (
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/repository"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	a.r.LoadHTMLGlob("templates/*.html")
	a.r.Static("/css", "./css")

	a.r.GET("/home", a.loadHome)
	a.r.GET("/home/:title", a.loadPage)
	a.r.GET("/home/new_resource", a.loadNewRes)
	a.r.GET("/home/add_resource", a.addResource)
	a.r.GET("/home/edit_resource", a.editResource)
	a.r.GET("/home/:title/add_monthly_prod", a.addMonthlyProd)
	//a.r.GET("/home/:resource_name/add_report", a.addReport)

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

	/*
		a.r.POST("/home/add_resource", func(c *gin.Context) {
			query := c.Params
			log.Println(query)
			log.Println("''''''''''''''''''''''''''''''''''''''")
		})

		a.r.GET("/home/new_resource/add_resource", a.loadNewRes)

	*/

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
		"ResourceName": resource.ResourceName,
		"Image":        resource.Image,
		"IsAvailable":  resource.IsAvailable,
		"Place":        resource.Place,
	})

}

func (a *Application) loadNewRes(c *gin.Context) {
	resource_name := c.Query("resource_name")
	fmt.Println(resource_name)
	c.HTML(http.StatusOK, "new_resource.html", gin.H{})

}

func (a *Application) addResource(c *gin.Context) {
	var requestBody ds.AddResRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	res := true
	for _, x := range []ds.Resources{} {
		if x.ResourceName == requestBody.ResourceName {
			log.Println(requestBody.ResourceName, " already exists")
			res = false
		}
	}

	if res == true {
		err := a.repo.AddResource(requestBody.ResourceName, requestBody.Place)
		log.Println(requestBody.ResourceName, " is added")

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ResourceName": requestBody.ResourceName,
			"Place":        requestBody.Place,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"ResourceName": requestBody.ResourceName,
			"message":      "already exists",
		})
	}
}

func (a *Application) editResource(c *gin.Context) {
	var requestBody ds.EditResNameRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	err := a.repo.EditResourceName(requestBody.OldName, requestBody.NewName)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"old name": requestBody.OldName,
		"new name": requestBody.NewName,
	})
}

func (a *Application) addMonthlyProd(c *gin.Context) {
	var requestBody ds.AddMonthlyProd
	resource_name := c.Param("title")

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	resources, _ := a.repo.GetAllResources()

	res := true
	for x := range resources {
		if resources[x].ResourceName == resource_name && resources[x].Place == requestBody.Place &&
			resources[x].Month == requestBody.Month {
			log.Println("Информация о добыче %s в месте %s за %s уже есть.",
				resource_name, requestBody.Place, requestBody.Month)
			res = false
		}
	}

	if res == true {
		err := a.repo.AddMonthlyProd(resource_name, requestBody.Place, requestBody.Month, requestBody.MonthlyProd)

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ResourceName": resource_name,
			"Place":        requestBody.Place,
			"Month":        requestBody.Month,
			"MonthlyProd":  requestBody.MonthlyProd,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"ResourceName": resource_name,
			"Place":        requestBody.Place,
			"Month":        requestBody.Month,
			"message":      "already exists",
		})
	}
}

/*
func (a *Application) addReport(c *gin.Context) {
	var requestBody ds.AddReportRequestBody

	resource_name := c.Param("resource_name")

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	err := a.repo()

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"old name": requestBody.OldName,
		"new name": requestBody.NewName,
	})
}

*/
