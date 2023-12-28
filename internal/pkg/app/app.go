package app

import (
	"ResourceExtraction/docs"
	"ResourceExtraction/internal/app/config"
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/redis"
	"ResourceExtraction/internal/app/repository"
	"ResourceExtraction/internal/app/role"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Application struct {
	repo   *repository.Repository
	r      *gin.Engine
	config *config.Config
	redis  *redis.Client
}

func New(ctx context.Context) (*Application, error) {
	cfg, err := config.NewConfig(ctx)
	if err != nil {
		return nil, err
	}

	repo, err := repository.New(dsn.FromEnv())
	if err != nil {
		return nil, err
	}

	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}

	return &Application{
		config: cfg,
		repo:   repo,
		redis:  redisClient,
	}, nil
}

// @title resources
// @version 0.0-0
// @description resource extraction

// @host localhost:8080
// @schemes http
// @BasePath /
func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	a.r.LoadHTMLGlob("templates/*.html")
	a.r.Static("/css", "./css")

	docs.SwaggerInfo.BasePath = "/"
	a.r.GET("swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	a.r.POST("/register", a.register)
	a.r.POST("/login", a.login)
	a.r.POST("/logout", a.logout)

	// доступ имеет только user
	//a.r.Use(a.WithAuthCheck(role.User)).GET("/ping", a.ping)
	a.r.GET("/resources", a.getAllResources)
	a.r.GET("/resources/:title", a.loadPage)

	a.r.POST("/async/:report_ref/:resource_ref", a.asyncInsertFact)

	clientMethods := a.r.Group("", a.WithAuthCheck(role.User))
	{
		//актуальное создание заявки + добавление в м-м
		clientMethods.POST("/reports/:title/add", a.addResourceToReport)

		//актуальное обновление записей в м-м

		clientMethods.POST("/reports/:title/delete", a.deleteReport)
		clientMethods.PUT("/reports/:title/add_data", a.addDataToReport)
		clientMethods.DELETE("/manage_reports/delete_single", a.deleteSingleFromMM)
		clientMethods.PUT("/manage_reports/add_plan", a.addPlanToMM)

	}

	moderMethods := a.r.Group("", a.WithAuthCheck(role.Admin))
	{
		moderMethods.PUT("/resources/:title/edit", a.editResource)
		moderMethods.POST("/resources/new_resource", a.addResource)
		moderMethods.DELETE("/resources/change_status/:title", a.deleteResource)
		moderMethods.GET("/ping", a.ping)
	}

	authorizedMethods := a.r.Group("", a.WithAuthCheck(role.User, role.Admin))
	{
		authorizedMethods.PUT("/reports/set_resources", a.setRequestOrbits)
		authorizedMethods.GET("/reports", a.getAllReports)
		authorizedMethods.GET("/reports/:title", a.getDetailedReport)
		authorizedMethods.GET("/reports/status/:status", a.getReportsByStatus)
		authorizedMethods.GET("/manage_reports/:title", a.getOrbitsFromTransfer)
		authorizedMethods.GET("/manage_reports/:title/extraction", a.getExtractionData)
		authorizedMethods.GET("/manage_reports/async_processed", a.getAsyncProcessed)
		authorizedMethods.PUT("/reports/change_status", a.changeStatus)
	}

	a.r.Run(":8000")

	log.Println("Server is down")
}

func (a *Application) asyncInsertFact(c *gin.Context) {
	log.Println("---")
	var requestBody = &ds.AsyncBody{}
	if err := c.BindJSON(&requestBody); err != nil {
		log.Println("ERROR")
		c.Error(err)
	}
	log.Println("ASYNC: ", requestBody.ReportID, " ---> ", requestBody.Fact)

	err := a.repo.AddResourceFactToMM(uint(requestBody.ReportID), uint(requestBody.ResourceID), requestBody.Fact)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, requestBody.Fact)
}

// @Summary Загрузка главной страницы
// @Description Загружает главную страницу с ресурсами или выполняет поиск ресурсов по названию.
// @Accept json
// @Tags Resources
// @Produce json
// @Param title query string false "Название ресурса для поиска"
// @Success 200 {object} map[string]interface{}
// @Router /home [get]
func (a *Application) loadHome(c *gin.Context) {
	title := c.Query("title") // для поиска

	allResources, err := a.repo.GetAllResources(title) // поиск либо по месту либо по названию
	log.Println(allResources)

	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusOK, allResources)

	/* ДЛЯ 4 ЛАБЫ:
	c.HTML(http.StatusOK, "hp_resources.html", gin.H{
		"materials": allRes,
	})

	*/
}

func (a *Application) getAllResources(c *gin.Context) {
	resourceName := c.Query("resourceName")
	highDemand := c.Query("highDemand")

	allOrbits, err := a.repo.GetAllResourcesNative(resourceName, highDemand)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, allOrbits)
}

// @Summary Загрузка страницы ресурса
// @Description Загружает страницу с определенным ресурсом и информацию о нем
// @Accept json
// @Tags Resources
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /home/{title} [get]
func (a *Application) loadPage(c *gin.Context) {
	resource_name := c.Param("title")
	log.Println(resource_name)
	if resource_name == "favicon.ico" {
		return
	}

	resource, err := a.repo.GetResourceByName(resource_name)

	if err != nil {
		c.Error(err)
		return
	}
	/*
		var months []string
		var monthlyProd []float64

			for i := range resources {
				months = append(months, resources[i].Month)
				monthlyProd = append(monthlyProd, resources[i].MonthlyProduction)
			}
			log.Println(months)
			log.Println(monthlyProd)
	*/
	c.JSON(http.StatusOK, gin.H{
		"ResourceName": resource.ResourceName,
		"Image":        resource.Image,
		"IsAvailable":  resource.IsAvailable,
		"Density":      resource.Density,
		"Demand":       resource.Demand,
		"IsToxic":      resource.IsToxic,
		"Desc":         resource.Description,
	})

	/*

		c.HTML(http.StatusOK, "rp_resource.html", gin.H{
				"ResourceName": resources[0].ResourceName,
				"Image":        resources[0].Image,
				"IsAvailable":  resources[0].IsAvailable,
				"Place":        resources[0].Place,
				"Months":       months,
				"MonthlyProds": monthlyProd,
			})


				ДЛЯ 4 ЛАБЫ: */
}

// @Summary Добавление нового ресурса
// @Description Добавляет новый ресурс с соответсвующими параметрами
// @Accept json
// @Tags Resources
// @Produce json
// @Param resource body ds.AddResRequestBody true "Ресурс"
// @Success 200 {object} map[string]interface{}
// @Router /home/add_resource [post]
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
		log.Println("1")
		err := a.repo.AddResource(requestBody.ResourceName, requestBody.Image, requestBody.Desc, requestBody.Density, requestBody.Demand, requestBody.IsToxic)
		log.Println(requestBody.ResourceName, " is added")

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ResourceName": requestBody.ResourceName,
			"IsAvailable":  true,
			"Density":      requestBody.Density,
			"IsToxic":      requestBody.IsToxic,
			"Demand":       requestBody.Demand,
			"Image":        requestBody.Image,
			"Desc":         requestBody.Desc,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"ResourceName": requestBody.ResourceName,
			"message":      "already exists",
		})
	}
}

// @Summary Удаление ресурса
// @Description Логически удаляет ресурс (меняет статус)
// @Accept json
// @Tags Resources
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /home/delete_resource/{title} [post]
func (a *Application) deleteResource(c *gin.Context) {
	resource_name := c.Param("title")
	err := a.repo.ChangeAvailability(resource_name)
	if err != nil {
		c.Error(err)
		return
	}

	c.Redirect(http.StatusOK, "/home")
	fmt.Println("redirected")
}

// @Summary Изменение данные о ресурсе
// @Description Можно изменить название, статус и картинку
// @Accept json
// @Tags Resources
// @Produce json
// @Param resource body ds.Resources true "Ресурс"
// @Success 200 {object} map[string]interface{}
// @Router /home/{title}/edit_resource [put]
func (a *Application) editResource(c *gin.Context) {
	resource_name := c.Param("title")
	resource, err := a.repo.GetResourceByName(resource_name)

	var editingResource ds.Resources

	if err := c.BindJSON(&editingResource); err != nil {
		c.Error(err)
	}

	err = a.repo.EditResource(resource.ResourceName, editingResource)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"NewName":        editingResource.ResourceName,
		"NewIsAvailable": editingResource.IsAvailable,
		"NewImage":       editingResource.Image,
		"NewDesc":        editingResource.Description,
		"NewDemand":      editingResource.Demand,
		"NewDensity":     editingResource.Density,
		"NewIsToxic":     editingResource.IsToxic,
	})
}

// @Summary Добавление информации о месячной добычи
// @Description Если записей о добыче ресурса еще нет, то изменяет эту запись. Если же информация о добыче за какие-то месяцы уже есть, то создает новую запись
// @Accept json
// @Tags Resources
// @Produce json
// @Param resource body ds.AddMonthlyProd true "Месячная доыбча"
// @Success 200 {object} map[string]interface{}
// @Router /home/{title}/add_monthly_prod [post]
/*
func (a *Application) addMonthlyProd(c *gin.Context) {
	var requestBody ds.AddMonthlyProd

	resource_name := c.Param("title")
	resName, _ := a.repo.GetResourceByName(resource_name)

	if err := c.BindJSON(&requestBody); err != nil {
		// error handler
	}

	resources, _ := a.repo.GetAllResources("")

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

}*/

// @Summary Добавление отчета о добыче
// @Description Добавление отчета по добыче по какому-то ресурса (по месту, в котором он добывается)
// @Accept json
// @Tags Resources
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /home/{title}/add_report [post]
/*
func (a *Application) addResourceToReport(c *gin.Context) {
	orbit_name := c.Param("title")

	// Получение инфы об орбите -> orbit.ID
	orbit, err := a.repo.GetResourceByName(orbit_name)
	if err != nil {
		c.Error(err)
		return
	}

	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}

	request := &ds.ExtractionReports{}
	request, err = a.repo.CreateNewReport(userUUID.(uuid.UUID))
	if err != nil {
		c.Error(err)
		return
	}

	err = a.repo.AddReportToMM(orbit.ID, request.ID)
	if err != nil {
		c.Error(err)
		return
	}
}
*/
func (a *Application) addResourceToReport(c *gin.Context) {
	orbit_name := c.Param("title")

	request_body := &ds.CreateReportBody{}
	if err := c.BindJSON(&request_body); err != nil {
		c.String(http.StatusBadGateway, "Не могу распознать json")
		return
	}
	log.Println(request_body)

	// Получение инфы об орбите -> orbit.ID
	orbit, err := a.repo.GetResourceByName(orbit_name)
	if err != nil {
		c.Error(err)
		return
	}

	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}

	request := &ds.ExtractionReports{}
	request, err = a.repo.CreateTransferRequest(userUUID.(uuid.UUID))
	if err != nil {
		c.Error(err)
		return
	}

	err = a.repo.AddReportToMM(orbit.ID, request.ID)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, request.ID)
}

func (a *Application) setRequestOrbits(c *gin.Context) {
	var requestBody ds.SetReportResourcesBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.String(http.StatusBadRequest, "Не получается распознать json запрос")
		return
	}

	err := a.repo.SetRequestOrbits(requestBody.ReportID, requestBody.Resources)
	if err != nil {
		c.String(http.StatusInternalServerError, "Не получилось задать ресурсы для заявки\n"+err.Error())
	}

	c.String(http.StatusCreated, "Ресурсы в заявке успешно заданы!")

}

func (a *Application) getAllReports(c *gin.Context) {
	dateStart := c.Query("date_start")
	dateFin := c.Query("date_fin")

	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}
	//userUUID, exists := c.Get("userUUID")
	//if !exists {
	//	panic(exists)
	//}

	requests, _, err := a.repo.GetAllRequests(userRole, dateStart, dateFin)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, requests)
}

// @Summary Получение отчета по его айди
// @Description Получение отчета по его айди
// @Accept json
// @Tags Reports
// @Produce json
// @Param title path string true "ID отчета"
// @Success 200 {object} map[string]interface{}
// @Router /home/get_report/{title} [get]
/*func (a *Application) getReportByID(c *gin.Context) {
	id := c.Param("title")
	id_uint, _ := strconv.ParseUint(id, 10, 64)

	report, _ := a.repo.GetReportByID(uint(id_uint))

	c.JSON(http.StatusOK, gin.H{
		"report id":    report.ID,
		"created at":   report.DateCreated,
		"client id":    report.ClientRef,
		"moderator id": report.ModeratorRef,
		"status":       report.Status,
	})
}*/

func (a *Application) getReportsByStatus(c *gin.Context) {
	req_status := c.Param("status")

	requests, err := a.repo.GetReportsByStatus(req_status)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (a *Application) getDetailedReport(c *gin.Context) {
	req_id, err := strconv.Atoi(c.Param("title"))
	if err != nil {
		log.Println("REQ ID: ", req_id)
		panic(err)
	}

	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}
	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}

	request, err := a.repo.GetReportByID(uint(req_id), userUUID.(uuid.UUID), userRole)
	if err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	c.JSON(http.StatusOK, request)
}

func (a *Application) deleteSingleFromMM(c *gin.Context) {
	var requestBody ds.DeletSingleFromMMBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}
	resource, err3 := a.repo.GetResourceByName(requestBody.ResourceName)
	if err3 != nil {
		return
	}
	parsedReqID, err4 := strconv.ParseUint(requestBody.RequestID, 10, 64)
	if err4 != nil {
		return
	}
	err1, err2 := a.repo.DeleteOneResourceFromMM(uint(parsedReqID), resource.ID)

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusCreated, "1 From MM was deleted")
}

// statuses for admin: на рассмотрении / отклонен / оказано
// statuses for user: черновик / удален

// @Summary Изменить статус у отчета
// @Description Изменение статуса у отчета с ограничениями
// @Accept json
// @Tags Reports
// @Produce json
// @Param title path string true "ID отчета"
// @Param change body ds.ChangeStatusRequestBody true "Кто меняет / на какой статус"
// @Success 200 {object} map[string]interface{}
// @Router /home/get_report/{title}/change_status [put]
func (a *Application) changeStatus(c *gin.Context) {
	var requestBody ds.ChangeStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}
	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}

	currRequest, err := a.repo.GetReportByID(requestBody.ReportID, userUUID.(uuid.UUID), userRole)
	if err != nil {
		c.Error(err)
		return
	}

	if !slices.Contains(ds.ReqStatuses, requestBody.Status) {
		c.String(http.StatusBadRequest, "Неверный статус")
		return
	}

	if userRole == role.User {
		if currRequest.ClientRef == userUUID {
			if slices.Contains(ds.ReqStatuses[:3], requestBody.Status) {
				err = a.repo.ChangeReportStatus(requestBody.ReportID, requestBody.Status)

				if err != nil {
					c.Error(err)
					return
				}

				c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
				return
			} else {
				c.String(http.StatusForbidden, "Клиент не может установить статус ", requestBody.Status)
				return
			}
		} else {
			c.String(http.StatusForbidden, "Клиент не является ответственным")
			return
		}
	} else {
		if currRequest.ModeratorRef == userUUID {
			if slices.Contains(ds.ReqStatuses[len(ds.ReqStatuses)-2:], requestBody.Status) {
				err = a.repo.ChangeReportStatus(requestBody.ReportID, requestBody.Status)

				if err != nil {
					c.Error(err)
					return
				}

				c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
				return
			} else {
				c.String(http.StatusForbidden, "Модератор не может установить статус ", requestBody.Status)
				return
			}
		} else {
			c.String(http.StatusForbidden, "Модератор не является ответственным")
			return
		}
	}
}

func (a *Application) getOrbitsFromTransfer(c *gin.Context) { // нужно добавить проверку на авторизацию пользователя
	req_id, err := strconv.Atoi(c.Param("title"))
	if err != nil {
		c.String(http.StatusBadRequest, "Ошибка в ID заявки")
		return
	}

	orbits, err := a.repo.GetOrbitsFromTransfer(req_id)
	log.Println(orbits)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка при получении ресурсов из заявки")
		return
	}

	c.JSON(http.StatusOK, orbits)

}

func (a *Application) getExtractionData(c *gin.Context) { // нужно добавить проверку на авторизацию пользователя
	req_id, err := strconv.Atoi(c.Param("title"))
	if err != nil {
		c.String(http.StatusBadRequest, "Ошибка в ID заявки")
		return
	}

	orbits, err := a.repo.GetExtractionDataByRepID(req_id)
	log.Println(orbits)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка при получении ресурсов из заявки")
		return
	}

	c.JSON(http.StatusOK, orbits)

}

func (a *Application) getAsyncProcessed(c *gin.Context) { // нужно добавить проверку на авторизацию пользователя
	n, err := a.repo.GetAsyncProcessedAmount()
	log.Println("обработано", n, "записей из М-М")
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка при получении ресурсов из заявки")
		return
	}

	c.JSON(http.StatusOK, n)

}

// меняем статус заявки на "удален" и физически удаляем состав заявки из ММ

// @Summary Удаление отчета
// @Description Логическое удаление отчета из таблицы отчетов и физическое от таблицы ММ
// @Accept json
// @Tags Reports
// @Produce json
// @Param title path string true "ID отчета"
// @Success 200 {object} map[string]interface{}
// @Router /home/delete_report/{title} [post]
func (a *Application) deleteReport(c *gin.Context) {
	req_id, err1 := strconv.Atoi(c.Param("req_id"))
	if err1 != nil {
		// ... handle error
		panic(err1)
	}

	err1, err2 := a.repo.DeleteReport(uint(req_id)), a.repo.DeleteAllResourcesFromMM(uint(req_id))

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusOK, "ExtractionReport & MM were deleted")
}

func (a *Application) addDataToReport(c *gin.Context) {
	req_id, err1 := strconv.Atoi(c.Param("title"))
	var requestBody ds.AddDataToReport
	if err1 != nil {
		// ... handle error
		panic(err1)
	}
	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}
	log.Println(requestBody.Place, requestBody.Month)
	err := a.repo.AddMonthPlaceToReport(uint(req_id), requestBody.Place, requestBody.Month)

	if err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusOK, "Added month and place to report")
}

func (a *Application) addPlanToMM(c *gin.Context) {
	var requestBody ds.AddPlanToMM

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}
	resRef, err1 := a.repo.GetResourceByName(requestBody.ResourceRef)
	if err1 == nil {
		err := a.repo.AddResourcePlanToMM(requestBody.ReportRef, resRef.ID, int(requestBody.Plan))

		if err != nil {
			c.Error(err)
			c.String(http.StatusBadRequest, "Bad Request")
			return
		}
	}

	c.String(http.StatusCreated, "Added plan to MM")
}

func (a *Application) deleteFromMM(c *gin.Context) {
	var requestBody ds.ManageReports

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	err1, err2 := a.repo.DeleteOneResourceFromMM(requestBody.ReportRef, requestBody.ResourceRef)

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusCreated, "Resource from ManageReports was deleted")
}

// ------------------------------------------------------------------------------------------------ //

type loginReq struct {
	Username string `json:"login"`
	Password string `json:"password"`
}

type loginResp struct {
	Username    string `json:"login"`
	Role        int    `json:"role"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type registerReq struct {
	Name string    `json:"login"` // лучше назвать то же самое что login
	Pass string    `json:"password"`
	Role role.Role `json:"role"`
}

type registerResp struct {
	Ok bool `json:"ok"`
}

// @Summary Загрузка страницы ресурса
// @Description Загружает страницу с определенным ресурсом и информацию о нем
// @Accept json
// @Tags Auth
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /ping [get]
func (a *Application) ping(gCtx *gin.Context) {
	log.Println("ping func")
	gCtx.JSON(http.StatusOK, gin.H{
		"auth": true,
	})
}

func (a *Application) register(gCtx *gin.Context) {
	req := &registerReq{}

	err := json.NewDecoder(gCtx.Request.Body).Decode(req)
	if err != nil {
		gCtx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if req.Pass == "" {
		gCtx.AbortWithError(http.StatusBadRequest, fmt.Errorf("pass is empty"))
		return
	}

	if req.Name == "" {
		gCtx.AbortWithError(http.StatusBadRequest, fmt.Errorf("name is empty"))
		return
	}

	err = a.repo.Register(&ds.Users{
		UUID:     uuid.New(),
		Role:     req.Role,
		Username: req.Name,
		Password: a.repo.GenerateHashString(req.Pass), // пароли делаем в хешированном виде и далее будем сравнивать хеши, чтобы их не угнали с базой вместе
	})
	if err != nil {
		gCtx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	gCtx.JSON(http.StatusOK, &registerResp{
		Ok: true,
	})
}

// @Summary Загрузка страницы ресурса
// @Description Загружает страницу с определенным ресурсом и информацию о нем
// @Accept json
// @Tags Auth
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /login [get]
func (a *Application) login(c *gin.Context) {
	cfg := a.config
	req := &loginReq{}
	err := json.NewDecoder(c.Request.Body).Decode(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)

		return
	}

	user, err := a.repo.GetUserByName(req.Username)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}
	fmt.Println(req.Username, user.Username, user.Password, a.repo.GenerateHashString(req.Password))
	if req.Username == user.Username && user.Password == a.repo.GenerateHashString(req.Password) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, &ds.JWTClaims{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(time.Second * 3600).Unix(), //1h
				IssuedAt:  time.Now().Unix(),
				Issuer:    "web-admin",
			},
			Role:     user.Role,
			UserUUID: user.UUID,
		})

		if token == nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Токен пустой"))

			return
		}

		strToken, err := token.SignedString([]byte(cfg.JWT.Token))
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Невозможно получить строку из токена"))

			return
		}

		//httpOnly=true, secure=true -> не могу читать куки на фронте ...
		c.SetCookie("resources-api-token", "Bearer "+strToken, int(time.Now().Add(time.Second*3600).
			Unix()), "", "", true, true)

		c.JSON(http.StatusOK, loginResp{
			Username:    user.Username,
			Role:        int(user.Role),
			AccessToken: strToken,
			TokenType:   "Bearer",
			ExpiresIn:   int(cfg.JWT.ExpiresIn.Seconds()),
		})
		log.Println("\nUSER: ", user.Username, "\n", strToken, "\n")
		c.AbortWithStatus(http.StatusOK)
	} else {
		c.AbortWithStatus(http.StatusForbidden)
	}
}

// @Summary Загрузка страницы ресурса
// @Description Загружает страницу с определенным ресурсом и информацию о нем
// @Accept json
// @Tags Auth
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /logout [get]
func (a *Application) logout(c *gin.Context) {
	jwtStr, err := GetJWTToken(c)
	if err != nil {
		panic(err)
	}

	if !strings.HasPrefix(jwtStr, jwtPrefix) {
		c.AbortWithStatus(http.StatusForbidden)

		return
	}

	jwtStr = jwtStr[len(jwtPrefix):]

	_, err = jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWT.Token), nil
	})
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Println(err)

		return
	}

	err = a.redis.WriteJWTToBlackList(c.Request.Context(), jwtStr, a.config.JWT.ExpiresIn)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	c.Status(http.StatusOK)
}
