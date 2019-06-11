package main

import (
	"context"
	"math"
	"regexp"
	"strconv"
	"time"

	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/ChimeraCoder/anaconda"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"github.com/volatiletech/sqlboiler/boil"

	"github.com/x1-/detect-engineer-client/models"
)

const (
	Delimiter             = "#DEMI#"
	Index                 = "index.html"
	ErrorGeneral          = "システムに問題が発生しました。"
	ErrorInvalidOperation = "不正な操作が行われました。"
	ErrorZeroTweet        = "ツイートが取得できませんでした。"
)

var (
	consumerKey       = flag.String("consumer_key", "", "Issued from Twitter.")
	consumerSecret    = flag.String("consumer_secret", "", "Issued from Twitter.")
	accessToken       = flag.String("access_token", "", "Issued from Twitter.")
	accessTokenSecret = flag.String("access_token_secret", "", "Issued from Twitter.")
	dbHost            = flag.String("db_host", "127.0.0.1", "The host address of database.")
	dbPort            = flag.String("db_port", "3306", "The host port of database.")
	dbName            = flag.String("db_name", "", "The name of database.")
	dbUser            = flag.String("db_user", "", "The user of accessing database.")
	dbPasswd          = flag.String("db_passwd", "", "The password of database user.")
	doc2vecEndpoint   = flag.String("doc2vec_endpoint", "", "An endpoint of doc2vec.")
	femaleEndpoint    = flag.String("female_endpoint", "", "An endpoint of female predictor.")
	engineerEndpoint  = flag.String("engineer_endpoint", "", "An endpoint of engineer predictor.")
)

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

	flag.Parse()

	ctx := context.Background()
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", *dbUser, *dbPasswd, *dbHost, *dbPort, *dbName))
	if err != nil {
		panic(err.Error())
	}
	boil.SetDB(db)
	boil.DebugMode = true

	twClient := getTwitterAPI()
	var server *gin.Engine

	server = gin.New()
	server.Use(gin.Logger())

	server.LoadHTMLGlob("templates/*.html")

	server.Static("/css/", "./public/css")
	server.Static("/js/", "./public/js/")
	server.Static("/img/", "./public/img/")
	server.Static("/vendors/", "./public/vendors/")

	server.GET("/", IndexRouter)
	server.POST("/", func(g *gin.Context) {
		AnalyzeRouter(ctx, g, twClient)
	})

	server.Run(":8001")
}

// IndexRouter index ページを表示します.
func IndexRouter(g *gin.Context) {
	g.HTML(http.StatusOK, Index, nil)
}

// AnalyzeRouter POSTされたデータを解析し、結果を表示します.
func AnalyzeRouter(ctx context.Context, g *gin.Context, api *anaconda.TwitterApi) {

	a, errors := validation(g)

	start := time.Now()
	isTweets := false
	if a.Account != "" {
		isTweets = true
		tweet := getTextFromAccount(api, a.Account)
		if len(tweet) == 0 {
			errors = append(errors, ErrorZeroTweet)
		}
		a.Tweet = tweet
	} else {
		a.Tweet = fmt.Sprintf("%s%s", convNewline(a.Tweet, ","), Delimiter)
	}

	var err error
	var doc2vec *Doc2Vec
	var female *LSTM

	if len(errors) == 0 {
		doc2vec, err = postDoc2vec(*doc2vecEndpoint, a.Tweet, "")
		if err != nil {
			errors = append(errors, ErrorGeneral)
		}
		log.Infof("res: %v", doc2vec)
	}
	if len(errors) == 0 {
		female, err = postFemalePredictor(*femaleEndpoint, doc2vec)
		if err != nil {
			errors = append(errors, ErrorGeneral)
		}
		log.Infof("res: %v", female)
	}

	fProbe, fPredict := alignProbe(female)

	err = addAccess(ctx, a, fPredict, fProbe, .0, .0)
	if err != nil {
		errors = append(errors, ErrorGeneral)
	}

	if isTweets {
		end := time.Now()
		d := end.Sub(start).Nanoseconds()
		wait := 1*1000*1000*1000 - d + (1 * 1000 * 1000)
		time.Sleep(time.Duration(wait))
	}

	if len(errors) > 0 {
		g.HTML(http.StatusOK, Index, gin.H{
			"errors":  errors,
			"account": a.Account,
			"tweet":   a.Tweet,
		})
		return
	}

	fProbe = probeByTrue(fPredict, fProbe)

	g.HTML(http.StatusOK, Index, gin.H{
		"analyzed":   true,
		"female":     fProbe,
		"female_p":   Round(fProbe*100.0, 2),
		"engineer":   0.12356,
		"engineer_p": 12.356,
	})
}

// postDoc2vec tweet を Doc2Vecモデルに問い合わせ、文章のベクトル表現を取得します.
func postDoc2vec(endpoint string, data string, urlType string) (*Doc2Vec, error) {

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
		log.Error("an error occured at postFemalePredictor.")
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

// postFemalePredictor Doc2Vecモデル から返ってきた文章のベクトル表現から男女を推定します.
func postFemalePredictor(endpoint string, data *Doc2Vec) (*LSTM, error) {

	d := data.Data
	d.Tensor.Shape = []int{1, 600}

	req := LSTMRequest{
		Data: d,
	}
	j, err := json.Marshal(req)
	if err != nil {
		log.Errorf("json.Marshal: %v", err)
		return nil, err
	}
	log.Debugf("json: %s", string(j))

	body, err := request(endpoint, string(j))
	if err != nil {
		log.Error("an error occured at postFemalePredictor.")
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

// request API にデータをポストします.
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

// Round 四捨五入
func Round(num, places float64) float64 {
	shift := math.Pow(10, places)
	return roundInt(num*shift) / shift
}

// roundInt 四捨五入(整数)
func roundInt(num float64) float64 {
	t := math.Trunc(num)
	if math.Abs(num-t) >= 0.5 {
		return t + math.Copysign(1, num)
	}
	return t
}

// addAccess access テーブルにデータを insert します.
func addAccess(ctx context.Context, a *models.Access, fPredict float64, fProbe float64, ePredict float64, eProbe float64) error {
	a.PredictedSex = int(fPredict)
	a.ProbabilitySex = fProbe
	a.PredictedEngineer = int(ePredict)
	a.ProbabilityEngineer = eProbe
	if a.Account == "" {
		a.Account = "-"
	}
	err := a.InsertG(ctx, boil.Blacklist("id"))
	if err != nil {
		log.Errorf("Failed to insert Access: %v", err)
		return err
	}
	return nil
}

// validation 入力値のバリデーションを行います.
func validation(g *gin.Context) (*models.Access, []string) {

	errors := make([]string, 0, 5)

	var a *models.Access
	if g.Bind(&a) != nil {
		errors = append(errors, ErrorInvalidOperation)
	}
	if len(a.Account) == 0 && len(a.Tweet) == 0 {
		errors = append(errors, "Twitter アカウント か つぶやき のどちらかを入力してください。")
	}

	return a, errors
}

// getTwitterAPI Twitter API Client を取得します.
func getTwitterAPI() *anaconda.TwitterApi {
	anaconda.SetConsumerKey(*consumerKey)
	anaconda.SetConsumerSecret(*consumerSecret)
	return anaconda.NewTwitterApi(*accessToken, *accessTokenSecret)
}
func getUserTimeline(api *anaconda.TwitterApi, scname string, count int) []anaconda.Tweet {

	var tweets = make([]anaconda.Tweet, 0, 10)

	sCount := strconv.Itoa(count)

	v := url.Values{}
	v.Set("screen_name", scname)
	v.Set("count", sCount)
	v.Set("exclude_replies", "true")
	v.Set("include_rts", "false")

	tws, _ := api.GetUserTimeline(v)
	tweets = append(tweets, tws...)

	return tweets
}
func getTextFromAccount(api *anaconda.TwitterApi, scname string) string {
	ss := make([]string, 0, 5)
	tweets := getUserTimeline(api, scname, 100)
	rep := regexp.MustCompile(`https?://[\w/:%#\$&\?\(\)~\.=\+\-]+`)

	for _, t := range tweets {
		urlType := getUrlType(t)
		if urlType != "external" {
			s := convNewline(t.FullText, ",")
			replaced := ".QUOTATION."
			if urlType == "photo" {
				replaced = ".PHOTOIMAGE."
			}
			ss = append(ss, rep.ReplaceAllString(s, replaced))
		}
	}
	// num := len(ss)
	return strings.Join(ss, ",")
}

func getUrlType(t anaconda.Tweet) string {
	for _, u := range t.Entities.Urls {
		if (u.Expanded_url != "") && (strings.Index(u.Expanded_url, "https://twitter.com") < 0) {
			return "external"
		}
	}
	urlType := ""
	for _, m := range t.Entities.Media {
		urlType = m.Type
	}
	return urlType
}

func convNewline(str, nlcode string) string {
	return strings.NewReplacer(
		"\r\n", nlcode,
		"\r", nlcode,
		"\n", nlcode,
	).Replace(str)
}

func probeByTrue(tf float64, v float64) float64 {
	nv := v
	if int(tf) != 1 {
		nv = float64(1.0) - v
	}
	return nv
}

func alignProbe(v *LSTM) (float64, float64) {
	var probe float64
	var predict float64
	if v != nil {
		probe = v.Data.Tensor.Values[0]
		predict = v.Data.Tensor.Values[1]
	} else {
		probe = .0
		predict = -1
	}
	return probe, predict
}
