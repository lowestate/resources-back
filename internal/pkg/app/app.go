package app

import (
	"ResourceExtraction/internal/app/ds"
	"ResourceExtraction/internal/app/dsn"
	"ResourceExtraction/internal/app/repository"
	"ResourceExtraction/internal/app/role"
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

// @title           Swagger Example API
// @version         1.0
// @description     This is a sample server celler server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /home

// @securityDefinitions.basic  BasicAuth

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/

func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	a.r.LoadHTMLGlob("templates/*.html")
	a.r.Static("/css", "./css")

	a.r.GET("swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	//a.r.POST("/sign_up", a.Register)
	//a.r.POST("/login", a.Login)
	// доступ имеет только user
	//a.r.Use(a.WithAuthCheck(role.User)).GET("/ping", a.ping)

	a.r.GET("/home", a.loadHome)
	a.r.GET("/home/:title", a.loadPage)
	//a.r.Use(a.WithAuthCheck(role.User)).GET("/home/:title", a.loadPage)
	a.r.GET("/home/new_resource", a.loadNewRes)
	a.r.POST("/home/add_resource", a.addResource)
	a.r.PUT("/home/edit_resource", a.editResource)
	a.r.POST("/home/:title/add_monthly_prod", a.addMonthlyProd)

	a.r.POST("/home/:title/add_report", a.addReport)
	a.r.GET("/home/get_report/:title", a.getReportByID)
	//a.r.Use(a.WithAuthCheck(role.Admin)).GET("/home/get_report/:title", a.getReportByID)
	a.r.PUT("/home/get_report/:title/change_status", a.changeStatus)
	a.r.POST("/home/change_res_status/:title", a.deleteResource)
	a.r.POST("/home/delete_report/:title", a.deleteReport)

	a.r.Run(":8080")

	log.Println("Server is down")
}

type Application struct {
	repo   repository.Repository
	r      *gin.Engine
	config struct {
		JWT struct {
			Token         string
			SigningMethod jwt.SigningMethod
			ExpiresIn     time.Duration
		}
	}
}

type registerReq struct {
	Name string    `json:"username"` // лучше назвать то же самое что login
	Pass string    `json:"password"`
	Role role.Role `json:"role"`
}

type registerResp struct {
	Ok bool `json:"ok"`
}

func (a *Application) Register(gCtx *gin.Context) {
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

	if req.Name == "" {
		gCtx.AbortWithError(http.StatusBadRequest, fmt.Errorf("no role chosen"))

		return
	}

	err = a.repo.Register(&ds.UserUuid{
		UUID:     uuid.New(),
		Username: req.Name,
		Role:     req.Role,
		Password: generateHashString(req.Pass), // пароли делаем в хешированном виде и далее будем сравнивать хеши, чтобы их не угнали с базой вместе
	})
	log.Println("new user registered")

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

const jwtPrefix = "Bearer "

func (a *Application) WithAuthCheck(assignedRoles ...role.Role) func(ctx *gin.Context) {
	log.Println("withauthcheck")
	return func(gCtx *gin.Context) {
		jwtStr := gCtx.GetHeader("Authorization")
		if !strings.HasPrefix(jwtStr, jwtPrefix) { // если нет префикса то нас дурят!
			gCtx.AbortWithStatus(http.StatusForbidden) // отдаем что нет доступа

			return // завершаем обработку
		}

		log.Println("prefix ok")

		// отрезаем префикс
		jwtStr = jwtStr[len(jwtPrefix):]

		token, err := jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(a.config.JWT.Token), nil
		})
		log.Println("token:", token)
		if err != nil {
			gCtx.AbortWithStatus(http.StatusForbidden)
			log.Println(err)

			return
		}

		myClaims := token.Claims.(*ds.JWTClaims)

		for _, oneOfAssignedRole := range assignedRoles {
			if myClaims.Role == oneOfAssignedRole {
				gCtx.AbortWithStatus(http.StatusOK)

				return
			}
		}

		gCtx.AbortWithStatus(http.StatusForbidden)
		log.Println("this role does not have enough rights")
	}
}

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

func (a *Application) Login(gCtx *gin.Context) {
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

// Ping godoc
// @Summary      Show hello text
// @Description  very very friendly response
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router       /ping/{name} [get]
func (a *Application) ping(gCtx *gin.Context) {
	gCtx.JSON(http.StatusOK, gin.H{
		"auth": true,
	})
}

func New() Application {
	app := Application{}

	repo, _ := repository.New(dsn.FromEnv())

	app.repo = *repo

	return app
}

// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router       /home [get]
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

		c.JSON(http.StatusOK, gin.H{
			"materials": allRes,
		})

		/* ДЛЯ 4 ЛАБЫ:
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"materials": allRes,
		})

		*/
	} else {
		foundResources, err := a.repo.SearchResources(resourceName)

		if err != nil {
			c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"materials": a.repo.UniqueResources(foundResources),
			"Material":  resourceName,
		})

		/*

			c.HTML(http.StatusOK, "hp_resources.html", gin.H{
					"materials": a.repo.FilteredResources(foundResources),
					"Material":  resourceName,
				})
					ДЛЯ 4 ЛАБЫ: */

	}
}

// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router       /home/{title} [get]
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

// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router       /ping/{name} [get]
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

// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router       /home/change_res_status/{title} [post]
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

// Ping godoc
// @Description  adding monthly prod
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router      /home/{title}/add_monthly_prod [post]
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

// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router   /home/{title}/add_report [post]
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

// / Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router   home/get_report/{title} [get]
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
// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router   /home/get_report/{title}/change_status [put]
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

// меняем статус заявки на "удален" и физически удаляем соста заявки из ММ
// Ping godoc
// @Description  adding new resource
// @Tags         Tests
// @Produce      json
// @Success      200  {object}  pingResp
// @Router   /home/delete_report/{title} [post]
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
