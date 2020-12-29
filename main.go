package main

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"os"
	"github.com/PuerkitoBio/goquery"
	"github.com/parnurzeal/gorequest"
	"gopkg.in/gomail.v2"
)

const (
	FundJsUrl    = "http://fundgz.1234567.com.cn/js/"
	FundHTMLUrl  = "http://fund.eastmoney.com/"
	MIN_RISE_NUM = 0.5
	MAX_FALL_NUM = -0.5
)

type Fund struct {
	code string
	name string
	memo string
}

func CreateFund(fundcode string, fundname string, fundmemo string) *Fund {
	return &Fund{code: fundcode, name: fundname, memo: fundmemo}
}

var fundCodeStr = "110022"
var fundCodeSlice = strings.Split(fundCodeStr, "|")
var date = ""

var dailyTitle = `
				 <tr>
				 <td width="50" align="center">分类</td>
				 <td width="50" align="center">基金代码</td>
				 <td width="200" align="left">基金名称</td>
				 <td width="50" align="center">估算涨幅</td>
				 <td width="50" align="center">当前估算净值</td>
				 <td width="50" align="center">昨日单位净值</td>
				 <td width="100" align="center">估算时间</td>
				 <td width="50" align="center">近1周净值变化</td>
				 <td width="50" align="center">近1月净值变化</td>
                 </tr>
                 `

var weeklyTitle = `
                 <tr>
	             <td width="200" align="left">基金名称</td>
	             <td width="50" align="center">近1周净值变化</td>
                 </tr>
                 `

var oneMonthTitle = `
                 <tr>
	             <td width="200" align="left">基金名称</td>
	             <td width="50" align="center">近1月净值变化</td>
                 </tr>
                 `

func FetchFund(funds []Fund) []map[string]string {
	var fundResult []map[string]string
	var weeklyChange string
	var oneMonthChange string
	for _, fund := range funds {
		fundJsUrl := FundJsUrl + fund.code + ".js"
		request := gorequest.New()
		resp, body, err := request.Get(fundJsUrl).End()
		defer resp.Body.Close()
		if err != nil {
			log.Fatal(err)
			return nil
		}

		fundHTMLUrl := FundHTMLUrl + fund.code + ".html"
		resp1, body1, err1 := request.Get(fundHTMLUrl).End()
		defer resp1.Body.Close()
		if err1 != nil {
			log.Fatal(err1)
			return nil
		}

		re, _ := regexp.Compile("jsonpgz\\((.*)\\);")
		ret := re.FindSubmatch([]byte(body))
		fundData := ret[1]

		doc, err2 := goquery.NewDocumentFromReader(strings.NewReader(body1))
		if err2 != nil {
			log.Fatal(err2)
		}
		doc.Find("#increaseAmount_stage > table:nth-child(1) > tbody:nth-child(1) > tr:nth-child(2)").Each(func(i int, s *goquery.Selection) {
			s.Find("td > div").Each(func(j int, k *goquery.Selection) {
				change := k.Text()
				switch j {
				case 1:
					weeklyChange = change
				case 2:
					oneMonthChange = change
				}
			})
		})
		var fundDataMap map[string]string
		if err := json.Unmarshal(fundData, &fundDataMap); err == nil {
			fundDataMap["weeklyChange"] = weeklyChange
			fundDataMap["oneMonthChange"] = oneMonthChange
			fundDataMap["memo"] = fund.memo
			fundResult = append(fundResult, fundDataMap)
		}
	}
	return fundResult
}

func GenerateHTML(fundResult []map[string]string) string {
	var dailyElements []string
	var dailyContent string
	var weeklyContent string
	var oneMonthContent string
	var dailyText string
	now := time.Now()
	for _, fund := range fundResult {
		gszzl, err := strconv.ParseFloat(fund["gszzl"], 32)
		if err != nil {
			fmt.Printf("!!error!!: %s", err)
			continue
		}
		if gszzl > 0 {
			fund["gszzl"] = "+" + strconv.FormatFloat(gszzl, 'f', -1, 32)
		}

		date = fund["gztime"]
		// 每日涨幅，涨跌幅度超出设定值才发出通知
		if (gszzl > 0 && gszzl >= 0.5) || (gszzl < 0 && gszzl <= -0.5) {
			dailyElement := `
				 <tr>
					 <td width="50" align="center">` + fund["memo"] + `</td>
					<td width="50" align="center">` + fund["fundcode"] + `</td>
					<td width="200" align="left">` + fund["name"] + `</td>
					<td width="50" align="center">` + fund["gszzl"] + `%</td>
					<td width="50" align="center">` + fund["gsz"] + `</td>
					<td width="50" align="center">` + fund["dwjz"] + `</td>
					<td width="100" align="center">` + fund["gztime"] + `</td>
					<td width="50" align="center">` + fund["weeklyChange"] + `</td>
					<td width="50" align="center">` + fund["oneMonthChange"] + `</td>			    
                                   </tr>
	                           `
		      dailyElements = append(dailyElements, dailyElement)
		}
	}
	dailyContent = strings.Join(dailyElements, "\n")

	if dailyContent != "" || weeklyContent != "" || oneMonthContent != "" {
		if dailyContent != "" {
			dailyText = `
                                    <table width="30%" border="1" cellspacing="0" cellpadding="0">
				    ` + dailyTitle + dailyContent + `
				    </table> <br><br>`
		}

		html := `
			</html>
			    <head>
			        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
			    </head>
                            <body>
			        <div id="container">
						<p>基金涨跌监控: 
						` + now.Format("2006-01-02 15:04:05") + `
						</p>
			            <div id="content">
				            ` + dailyText + `
				    </div>
            	                </div>
                            </body>
                        </html>`

		return html
	}

	return ""
}

func SendEmail(content string) {
	if content == "" {
		return
	}
	emailName := os.Getenv("EMAIL_NAME")
	emailPassword := os.Getenv("EMAIL_PASSWORD")
	m := gomail.NewMessage()
	m.SetHeader("From", emailName)
	m.SetHeader("To", emailName)
	m.SetHeader("Subject", "TaoTalk-基金涨跌监控 ["+date+"] + From travis-ci")
	m.SetBody("text/html", content)
	d := gomail.NewDialer("smtp.qq.com", 587, emailName, emailPassword)
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
}

func main() {
	funds := []Fund{
		Fund{"163417", "yy", "WT"},
		Fund{"005827", "yy", "WT"},
		Fund{"000751", "yy", "WT"},
		Fund{"519772", "yy", "WT"},
		Fund{"000083", "yy", "WT"},
		Fund{"001171", "yy", "WT"},
		Fund{"161028", "yy", "WT"},

		Fund{"000751", "yy", "成长投资"},
		Fund{"161005", "yy", "成长投资"},
		Fund{"260108", "yy", "成长投资"},
		Fund{"163406", "yy", "成长投资"},
		Fund{"519704", "yy", "成长投资"},
		Fund{"001875", "yy", "成长投资"},
		Fund{"040035", "yy", "成长投资"},
		Fund{"163415", "yy", "成长投资"},
		Fund{"202023", "yy", "成长投资"},
		Fund{"007119", "yy", "成长投资"},
		Fund{"005827", "yy", "价值投资"},
		Fund{"110022", "yy", "价值投资"},
		Fund{"180012", "yy", "价值投资"},
		Fund{"000083", "yy", "价值投资"},
		Fund{"162605", "yy", "价值投资"},
		Fund{"519066", "yy", "价值投资"},
		Fund{"213001", "yy", "价值投资"},
		Fund{"110003", "yy", "价值投资"},
		Fund{"519697", "yy", "价值投资"},
		Fund{"270002", "yy", "价值投资"},
		Fund{"202101", "yy", "稳健投资"},
		Fund{"110027", "yy", "稳健投资"},
		Fund{"485111", "yy", "稳健投资"},
		Fund{"003095", "yy", "成长投资***"},
		Fund{"001410", "yy", "成长投资***"},
	}
	fundResult := FetchFund(funds)
	content := GenerateHTML(fundResult)
	SendEmail(content)
}
