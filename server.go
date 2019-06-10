package main

import (
	// "context"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

const (
	Index                 = "index.html"
	ErrorGeneral          = "システムに問題が発生しました。"
	ErrorInvalidOperation = "不正な操作が行われました。"
)

type Analyzer struct {
	Account string `form:"account"`
	Tweet   string `form:"tweet"`
}

type Doc2VecRequest struct {
	StrData string `json:"strData"`
}
type LSTMRequest struct {
	Data Data `json:"data"`
}

type Doc2Vec struct {
	Data Data `json:"data"`
}
type LSTM struct {
	Data Data `json:"data"`
}

type Data struct {
	Tensor Tensor `json:"tensor"`
}
type Tensor struct {
	Shape  []int     `json:"shape"`
	Values []float64 `json:"values"`
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

	a, errors := validation(g)

	var err error
	var doc2vec *Doc2Vec
	var lstm *LSTM

	if len(errors) == 0 {
		doc2vec, err = postDoc2vec("http://*****:5000/predict", a.Tweet)
		if err != nil {
			errors = append(errors, ErrorGeneral)
		}
		log.Debugf("res: %v", doc2vec)
	}
	if len(errors) == 0 {
		lstm, err = postLSTM("http://*****:5000/predict", doc2vec)
		if err != nil {
			errors = append(errors, ErrorGeneral)
		}
		log.Infof("res: %v", lstm)
	}

	g.HTML(http.StatusOK, Index, gin.H{
		"analyzed":   true,
		"female_p":   0.37,
		"engineer_p": 0.11,
	})
}

func postDoc2vec(endpoint string, data string) (*Doc2Vec, error) {

	req := Doc2VecRequest{
		StrData: data,
	}
	j, err := json.Marshal(req)
	if err != nil {
		log.Errorf("json.Marshal: %v", err)
		return nil, err
	}

	body, err := request(endpoint, string(j))
	if err != nil {
		return nil, err
	}

	var doc2vec *Doc2Vec
	err = json.Unmarshal(body, &doc2vec)
	if err != nil {
		log.Errorf("json.Unmarshal: %v", err)
		return nil, err
	}
	return doc2vec, nil
}

func postLSTM(endpoint string, data *Doc2Vec) (*LSTM, error) {

	req := LSTMRequest{
		Data: data.Data,
	}
	j, err := json.Marshal(req)
	if err != nil {
		log.Errorf("json.Marshal: %v", err)
		return nil, err
	}
	log.Debugf("json: %s", string(j))

	body, err := request(endpoint, string(j))
	if err != nil {
		return nil, err
	}

	var lstm *LSTM
	err = json.Unmarshal(body, &lstm)
	if err != nil {
		log.Errorf("json.Unmarshal: %v", err)
		return nil, err
	}
	return lstm, nil
}

func request(endpoint string, data string) ([]byte, error) {

	values := url.Values{}
	values.Set("json", data)

	req, err := http.NewRequest(
		"POST",
		endpoint,
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		log.Errorf("request.NewRequest: %v", err)
		return nil, err
	}
	// Content-Type 設定
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Errorf("ioutil.ReadAllioutil.ReadAll.client.Do: %v", err)
		return nil, err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("response: %v", res.StatusCode)
		log.Error(err)
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("ioutil.ReadAll.ioutil.ReadAll: %v", err)
		return nil, err
	}
	return body, nil
}

func validation(g *gin.Context) (*Analyzer, []string) {

	errors := make([]string, 0, 5)

	var a *Analyzer
	if g.Bind(&a) != nil {
		errors = append(errors, ErrorInvalidOperation)
	}
	if len(a.Account) == 0 && len(a.Tweet) == 0 {
		errors = append(errors, "Twitter アカウント か つぶやき のどちらかを入力してください。")
	}

	return a, errors
}
