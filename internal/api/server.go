package api

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Resource struct {
	Title    string
	Material string
	Text     string
	Period   string
	Place    string
	ImageURL string
}

var materials = []Resource{
	{"titanium", "Титан", "2 кг", "01.01.2023 - 01.06.2023", "Море Восточное", "/image/titanium.png"},
	{"aluminium", "Алюминий", "11 кг", "12.02.2023 - 25.05.2023", "Океан Бурь", "/image/aluminium.jpg"},
	{"ferrum", "Железо", "6 кг", "15.01.2023 - 07.04.2023", "Море Влажности", "/image/ferrum.jpg"},
	{"regolith", "Реголит", "3 кг", "21.03.2023 - 09.05.2023", "Океан Бурь", "/image/regolith.jpg"},
	{"thorium", "Торий", "23 кг", "10.02.2023 - 16.07.2023", "Море Восточное", "/image/thorium.jpg"},
	{"uranium", "Уран", "18 кг", "06.02.2023 - 20.06.2023", "Океан Бурь", "/image/uranium.jpg"},
}

func StartServer() {
	log.Println("Server start up")

	r := gin.Default()
	r.Static("/css", "./css")
	// r - сущность (структура) типа Engine* с встроенным логгером* и Recovery middleware*
	// Engine - сущность фреймворка с muxer'ом (это мультиплексор HTTP запросов),
	//								  конфигурацией и миддлварой (слой обработки ошибок)

	// c *gin.Context отвечает за передачу данных между миддлварами
	//							  проверку того, что json приходит в нужном формате
	//							  рендер json ответа
	// H - сокращение от map[string]any

	r.GET("/home", loadHome)

	r.GET("/home/:title", loadPage)

	r.LoadHTMLGlob("templates/*") // подгружаем html файлы из templates

	r.Static("/image", "./resources") // это нужно чтобы картинки грузились ?

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	log.Println("Server down")
}

func loadHome(c *gin.Context) {
	queryParam, ok := c.GetQuery("search")

	if queryParam == "титан" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res1":  "Титан",
			"btn":   1,
		})
	} else if queryParam == "алюминий" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res2":  "Алюминий",
			"btn":   1,
		})
	} else if queryParam == "железо" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res3":  "Железо",
			"btn":   1,
		})
	} else if queryParam == "реголит" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res4":  "Реголит",
			"btn":   1,
		})
	} else if queryParam == "торий" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res5":  "Торий",
			"btn":   1,
		})
	} else if queryParam == "уран" && ok {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res6":  "Уран",
			"btn":   1,
		})
	} else {
		c.HTML(http.StatusOK, "hp_resources.html", gin.H{
			"title": "Ресурсы:",
			"res1":  "Титан",
			"res2":  "Алюминий",
			"res3":  "Железо",
			"res4":  "Реголит",
			"res5":  "Торий",
			"res6":  "Уран",
			"btn":   0,
		})
	}
}

func loadPage(c *gin.Context) {
	title := c.Param("title")
	for i := range materials {
		if materials[i].Title == title {
			c.HTML(http.StatusOK, "rp_resource.html", gin.H{
				"Material": materials[i].Material,
				"Text":     materials[i].Text,
				"Period":   materials[i].Period,
				"Place":    materials[i].Place,
				"ImageURL": materials[i].ImageURL, // URL-адрес изображения для Железа
			})
			return
		}
	}
}
