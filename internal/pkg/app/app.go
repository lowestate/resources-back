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
	"crypto/sha1"
	"encoding/hex"
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

	a.r.POST("/sign_up", a.register)
	a.r.POST("/login", a.login)
	a.r.POST("/logout", a.logout)
	// доступ имеет только user
	//a.r.Use(a.WithAuthCheck(role.User)).GET("/ping", a.ping)
	resourcesGroup := a.r.Group("/resources")
	{
		resourcesGroup.GET("", a.loadHome)
		resourcesGroup.GET("/:title", a.loadPage)
		resourcesGroup.POST("/change_res_status/:title", a.deleteResource)

		resourcesGroup.Use(a.WithAuthCheck(role.Admin))
		{
			resourcesGroup.POST("/add_resource", a.addResource)
			resourcesGroup.PUT("/:title/edit_resource", a.editResource)
			resourcesGroup.POST("/:title/add_monthly_prod", a.addMonthlyProd)
			resourcesGroup.POST("/:title/add_report", a.addReport)

		}
	}

	reportsGroup := a.r.Group("/reports")
	{
		reportsGroup.Use(a.WithAuthCheck(role.Admin))
		{
			reportsGroup.GET("", a.getAllReports)
			reportsGroup.GET("/get_report/:title", a.getReportByID)
			reportsGroup.PUT("/get_report/:title/change_status", a.changeStatus)
			reportsGroup.POST("/delete_report/:title", a.deleteReport)
		}
	}

	a.r.Run(":8080")

	log.Println("Server is down")
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

	c.JSON(http.StatusOK, gin.H{
		"ResourceName": resources[0].ResourceName,
		"Image":        resources[0].Image,
		"IsAvailable":  resources[0].IsAvailable,
		"Place":        resources[0].Place,
		"Months":       months,
		"MonthlyProds": monthlyProd,
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
		err := a.repo.AddResource(requestBody.ResourceName, requestBody.Place, requestBody.Image)
		log.Println(requestBody.ResourceName, " is added")

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"ResourceName": requestBody.ResourceName,
			"Place":        requestBody.Place,
			"Image":        requestBody.Image,
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

	c.Redirect(http.StatusFound, "/home")
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
	resources, err := a.repo.GetResourcesByName(resource_name)

	var editingResource ds.Resources

	if err := c.BindJSON(&editingResource); err != nil {
		c.Error(err)
	}

	err = a.repo.EditResource(resources[0].ResourceName, editingResource)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"NewName":        editingResource.ResourceName,
		"NewIsAvailable": editingResource.IsAvailable,
		"NewPlace":       editingResource.Place,
		"NewMonth":       editingResource.Month,
		"NewMonthlyProd": editingResource.MonthlyProduction,
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

}

// @Summary Добавление отчета о добыче
// @Description Добавление отчета по добыче по какому-то ресурса (по месту, в котором он добывается)
// @Accept json
// @Tags Resources
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /home/{title}/add_report [post]
func (a *Application) addReport(c *gin.Context) {
	resource_name := c.Param("title")

	resource, err := a.repo.GetResourceByName(resource_name)
	log.Println(resource)
	if err != nil {
		c.Error(err)
		return
	}

	request := &ds.ExtractionReports{}
	request, err = a.repo.CreateNewReport(0, resource_name) // user vasya
	log.Println("new request created")
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
		"new report added": request.ID,
	})
}

func (a *Application) getAllReports(c *gin.Context) {
	dateStart := c.Query("date_start")
	dateFin := c.Query("date_fin")

	userRole, exists := c.Get("role")
	if !exists {
		panic(exists)
	}
	//userUUID, exists := c.Get("userUUID")
	//if !exists {
	//	panic(exists)
	//}

	requests, err := a.repo.GetAllRequests(userRole, dateStart, dateFin)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusFound, requests)
}

// @Summary Получение отчета по его айди
// @Description Получение отчета по его айди
// @Accept json
// @Tags Reports
// @Produce json
// @Param title path string true "ID отчета"
// @Success 200 {object} map[string]interface{}
// @Router /home/get_report/{title} [get]
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
	})
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

// ------------------------------------------------------------------------------------------------ //

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Username    string
	Role        role.Role
	ExpiresIn   time.Duration `json:"expires_in"`
	AccessToken string        `json:"access_token"`
	TokenType   string        `json:"token_type"`
}

type registerReq struct {
	Name string    `json:"username"` // лучше назвать то же самое что login
	Pass string    `json:"password"`
	Role role.Role `json:"role"`
}

type registerResp struct {
	Ok bool `json:"ok"`
}

type pingReq struct{}
type pingResp struct {
	Status string `json:"status"`
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

	err = a.repo.Register(&ds.UserUuid{
		UUID:     uuid.New(),
		Role:     role.User,
		Username: req.Name,
		Password: generateHashString(req.Pass), // пароли делаем в хешированном виде и далее будем сравнивать хеши, чтобы их не угнали с базой вместе
	})
	if err != nil {
		gCtx.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	gCtx.JSON(http.StatusOK, &registerResp{
		Ok: true,
	})
}

func generateHashString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// @Summary Загрузка страницы ресурса
// @Description Загружает страницу с определенным ресурсом и информацию о нем
// @Accept json
// @Tags Auth
// @Produce json
// @Param title path string true "Название ресурса"
// @Success 200 {object} map[string]interface{}
// @Router /login [get]
func (a *Application) login(gCtx *gin.Context) {
	log.Println("login")
	cfg := a.config
	req := &loginReq{}

	err := json.NewDecoder(gCtx.Request.Body).Decode(req)
	if err != nil {
		gCtx.AbortWithError(http.StatusBadRequest, err)

		return
	}

	user, err := a.repo.GetUserByLogin(req.Username)
	log.Println("найден челик", req.Username, "-->", user.Username)
	if err != nil {
		gCtx.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	if req.Username == user.Username && user.Password == generateHashString(req.Password) {
		// значит проверка пройдена
		log.Println("проверка пройдена")
		// генерируем ему jwt
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, &ds.JWTClaims{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(time.Second * 3600).Unix(),
				IssuedAt:  time.Now().Unix(),
				Issuer:    "web-admin",
			},
			UserUUID: uuid.New(), // test uuid
			Role:     user.Role,
		})
		if token == nil {
			gCtx.AbortWithError(http.StatusInternalServerError, fmt.Errorf("token is nil"))

			return
		}

		strToken, err := token.SignedString([]byte(cfg.JWT.Token))
		if err != nil {
			gCtx.AbortWithError(http.StatusInternalServerError, fmt.Errorf("cant create str token"))

			return
		}

		gCtx.SetCookie("resources-api-token", "Bearer "+strToken, int(time.Now().Add(time.Second*3600).Unix()), "", "", true, false)

		log.Println(strToken)
		gCtx.JSON(http.StatusOK, loginResp{
			Username:    user.Username,
			Role:        user.Role,
			AccessToken: strToken,
			TokenType:   "Bearer",
			ExpiresIn:   cfg.JWT.ExpiresIn,
		})

		gCtx.AbortWithStatus(http.StatusOK)
	} else {
		gCtx.AbortWithStatus(http.StatusForbidden) // отдаем 403 ответ в знак того что доступ запрещен
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
	// получаем заголовок

	jwtStr, err := GetJWTToken(c)
	if err != nil {
		panic(err)
	}

	if !strings.HasPrefix(jwtStr, jwtPrefix) { // если нет префикса то нас дурят!
		c.AbortWithStatus(http.StatusForbidden) // отдаем что нет доступа

		return // завершаем обработку
	}

	// отрезаем префикс
	jwtStr = jwtStr[len(jwtPrefix):]

	_, err = jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWT.Token), nil
	})
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Println(err)

		return
	}

	// сохраняем в блеклист редиса
	err = a.redis.WriteJWTToBlackList(c.Request.Context(), jwtStr, a.config.JWT.ExpiresIn)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	c.Status(http.StatusOK)
}
