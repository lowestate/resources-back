package app

import (
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/repository"
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"slices"
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
	a.r.POST("/home/add_resource", a.addResource)
	a.r.PUT("/home/edit_resource", a.editResource)
	a.r.POST("/home/:title/add_monthly_prod", a.addMonthlyProd)
	a.r.POST("/home/:title/add_report", a.addReport)
	a.r.GET("/home/get_report/:title", a.getReportByID)
	a.r.PUT("/home/get_report/:title/change_status", a.changeStatus)
	a.r.POST("/home/delete_resource/:title", a.deleteResource)
	a.r.POST("/home/delete_report/:title", a.deleteReport)

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
	resourceName := c.DefaultQuery("title", "")
	fmt.Println(resourceName)

	if resourceName == "" {
		allRes, err := a.repo.GetAllResources()

		// получаем массив уникальных ресурсов по названию для главной страницы:
		allRes = a.repo.UniqueResources(allRes)

		if err != nil {
			c.Error(err)
		}

		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"materials": allRes,
		})

		/* ДЛЯ 4 ЛАБЫ:
		c.JSON(http.StatusOK, gin.H{
			"materials": allRes,
		})

		*/
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

		/*
			c.JSON(http.StatusOK, gin.H{
				"materials": a.repo.UniqueResources(foundResources),
				"Material":  resourceName,
			})

				ДЛЯ 4 ЛАБЫ: */

	}
}

func (a *Application) loadPage(c *gin.Context) {
	resource_name := c.Param("title")

	if resource_name == "favicon.ico" {
		return
	}

	resources, err := a.repo.GetResourcesByName(resource_name)

	if err != nil {
		c.Error(err)
		return
	}

	var months []string
	var monthlyProd []float64

	for i := range resources {
		months = append(months, resources[i].Month)
		monthlyProd = append(monthlyProd, resources[i].MonthlyProduction)
	}
	log.Println(months)
	log.Println(monthlyProd)

	c.HTML(http.StatusOK, "rp_resource.html", gin.H{
		"ResourceName": resources[0].ResourceName,
		"Image":        resources[0].Image,
		"IsAvailable":  resources[0].IsAvailable,
		"Place":        resources[0].Place,
		"Months":       months,
		"MonthlyProds": monthlyProd,
	})
	/*
		c.JSON(http.StatusOK, gin.H{
			"ResourceName": resources[0].ResourceName,
			"Image":        resources[0].Image,
			"IsAvailable":  resources[0].IsAvailable,
			"Place":        resources[0].Place,
			"Months":       months,
			"MonthlyProds": monthlyProd,
		})

			ДЛЯ 4 ЛАБЫ: */
}

func (a *Application) loadNewRes(c *gin.Context) {
	resource_name := c.Query("resource_name")
	fmt.Println(resource_name)
	//c.JSON(http.StatusOK, gin.H{
	//		"curr resource": resource_name,
	//	})

	c.HTML(http.StatusOK, "new_resource.html", gin.H{
		"curr resource": resource_name,
	})

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

func (a *Application) deleteResource(c *gin.Context) {
	resource_name := c.Param("title")
	err := a.repo.ChangeAvailability(resource_name)
	if err != nil {
		c.Error(err)
		return
	}

	c.Redirect(http.StatusFound, "/home")
	fmt.Println("redirected")
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
	request, err = a.repo.CreateNewReport(0, resource_name) // user vasya
	if err != nil {
		c.Error(err)
		return
	}
	log.Println("REQUEST ID: ", request.ID, "\nRes ID: ", resource.ID)

	err = a.repo.AddReport(int(resource.ID), int(request.ID))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"new report added": request.Place,
	})
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

	adminStatuses := []string{"на рассмотрении", "оказано", "отменено"}
	userStatuses := []string{"удалено", "черновик"}

	if report != nil {
		if (requestBody.Who == "admin") && (slices.Contains(adminStatuses, requestBody.New_status)) ||
			((requestBody.Who == "user") && (slices.Contains(userStatuses, requestBody.New_status))) {
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
				"error":              "incorrect data. see the correct data variants below",
				"users":              "admin, user",
				"statuses for admin": "на рассмотрении, отклонено, оказано",
				"statuses for user":  "черновик, удалено",
			})
		}
	}
}

func (a *Application) deleteReport(c *gin.Context) {
	report_ref := c.Param("title")
	id_report, _ := strconv.ParseUint(report_ref, 10, 64)

	report, _ := a.repo.GetReportByID(uint(id_report))
	if report != nil {
		err1, err2 := a.repo.DeleteReport(uint(id_report))
		if err1 != nil || err2 != nil {
			c.Error(err1)
			c.Error(err2)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"deleted report with ID": id_report,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"no report with id": id_report,
		})
	}
}
