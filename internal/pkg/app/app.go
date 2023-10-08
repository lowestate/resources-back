package app

import (
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/repository"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strconv"
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
	a.r.GET("/home/:title/add_report", a.addReport)
	a.r.GET("/home/get_report/:title", a.getReportByID)
	a.r.GET("/home/get_report/:title/change_status", a.changeStatus)

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
	resName, _ := a.repo.GetResourceByName(resource_name)

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	resources, _ := a.repo.GetAllResources()

	res := true
	for x := range resources {
		if resources[x].ResourceName == resource_name && resources[x].Month == requestBody.Month {
			log.Println("Информация о добыче %s в месте %s за %s уже есть.",
				resource_name, requestBody.Month)
			res = false
		}
	}
	if res == true {
		err := a.repo.AddMonthlyProd(resource_name, resName.Place, requestBody.Month, requestBody.MonthlyProd)

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ResourceName": resource_name,
			"Month":        requestBody.Month,
			"MonthlyProd":  requestBody.MonthlyProd,
			"message":      "added monthly prod",
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"ResourceName": resource_name,
			"Place":        resName.Place,
			"Month":        requestBody.Month,
			"message":      "already exists",
		})
	}

}

func (a *Application) addReport(c *gin.Context) {
	resource_name := c.Param("title")

	//получение инфы об орбите (id)
	resource, err := a.repo.GetResourceByName(resource_name)
	log.Println(resource_name)
	if err != nil {
		c.Error(err)
		return
	}

	request := &ds.ExtractionReports{}
	request, err = a.repo.CreateTransferRequest(0, resource_name) // user vasya
	if err != nil {
		c.Error(err)
		return
	}
	log.Println("REQUEST ID: ", request.ID, "\nRes ID: ", resource.ID)

	err = a.repo.AddTransferToOrbits(int(resource.ID), int(request.ID))
	if err != nil {
		c.Error(err)
		return
	}

}

func (a *Application) getReportByID(c *gin.Context) {
	id := c.Param("title")
	id_uint, _ := strconv.ParseUint(id, 10, 64)

	report, _ := a.repo.GetReportByID(uint(id_uint))

	c.JSON(http.StatusOK, gin.H{
		"report id":    report.ID,
		"created at":   report.DateCreated,
		"client id":    report.ClientRef,
		"moderator id": report.ModeratorRef,
		"status":       report.Status,
		"place":        report.Place,
	})
}

// statuses for admin: на рассмотрении / отклонен / оказано
// statuses for user: черновик / удален
func (a *Application) changeStatus(c *gin.Context) {
	id := c.Param("title")
	id_uint, _ := strconv.ParseUint(id, 10, 64)

	var requestBody ds.ChangeStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	report, _ := a.repo.GetReportByID(uint(id_uint))

	if report != nil {
		if (requestBody.New_status == "удалено") ||
			(requestBody.New_status == "на рассмотрении") ||
			(requestBody.New_status == "отклонено") ||
			(requestBody.New_status == "оказано") ||
			(requestBody.New_status == "черновик") {

			if ((requestBody.Who == "admin") && ((requestBody.New_status == "на рассмотрении") ||
				(requestBody.New_status == "отклонено") ||
				(requestBody.New_status == "оказано"))) ||
				((requestBody.Who == "user") && (requestBody.New_status == "удалено") ||
					(requestBody.New_status == "черновик")) {
				err := a.repo.EditStatus(uint(id_uint), requestBody.New_status)

				if err != nil {
					c.Error(err)
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"id report":  id,
					"new status": requestBody.New_status,
				})
			} else {
				c.JSON(http.StatusOK, gin.H{
					"error":              "not enough rights to change to this status. see available statuses below.",
					"statuses for admin": "на рассмотрении, отклонено, оказано",
					"statuses for user":  "черновик, удалено",
				})
			}

		} else {
			c.JSON(http.StatusOK, gin.H{
				"error": "unknown status",
			})
		}
	} else {
		c.JSON(http.StatusOK, gin.H{
			"error": "report not found",
		})
	}

}
