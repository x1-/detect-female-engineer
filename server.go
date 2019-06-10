package main

import (
	// "context"

	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	Index = "index.html"
)

type Analyzer struct {
	Account string `form:"account"`
	Tweet   string `form:"tweet"`
}

func main() {

	var server *gin.Engine

	// ctx := context.Background()

	server = gin.New()
	server.Use(gin.Logger())

	server.LoadHTMLGlob("templates/*.html")

	server.Static("/css/", "./public/css")
	server.Static("/js/", "./public/js/")
	server.Static("/img/", "./public/img/")
	server.Static("/vendors/", "./public/vendors/")

	server.GET("/", IndexRouter)
	server.POST("/", AnalyzeRouter)

	server.Run(":8001")
}

func IndexRouter(g *gin.Context) {
	g.HTML(http.StatusOK, Index, nil)
}

func AnalyzeRouter(g *gin.Context) {

	var a Analyzer
	g.Bind(&a)

	log.Infof("post: %#v")

	g.HTML(http.StatusOK, Index, gin.H{
		"analyzed":   true,
		"female_p":   0.37,
		"engineer_p": 0.11,
	})
}
