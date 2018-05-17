package elm

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/pengkebao/elm/cache"
)

var appId string

var secretKey string

var apiUrl string

func init() {
	appId = "1a1b0136-a003-40e8-805d-6a5f53e29a1c"
	secretKey = "eb9dfe98-68a1-4204-806f-a3d69434daa6"
	apiUrl = "https://exam-anubis.ele.me"
	opts := &cache.RedisOpts{
		Host: "127.0.0.1:6379",
	}
	DB = cache.NewRedis(opts)
}

var DB *cache.Redis

type ELM struct {
	Code string      `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func NewElm() *ELM {
	return &ELM{}
}

func (this *ELM) Init(appid string, secretkey string, exam bool, redisHost string) {
	if exam == false {
		apiUrl = "https://open-anubis.ele.me"
	}
	if len(appId) <= 0 {
		fmt.Println("appId error")
	}
	if len(secretKey) <= 0 {
		fmt.Println("secretKey error")
	}
	appId = appid
	secretKey = secretkey
	opts := &cache.RedisOpts{
		Host: redisHost,
	}
	DB = cache.NewRedis(opts)
	//主动刷新AccessToken
	//加入任务
	if elmExpirTime, ok := DB.Get("ElmExpirTime").(float64); ok {
		ElmExpirTime := int64(elmExpirTime)
		if ElmExpirTime > (time.Now().Unix() - 60) {
			time.AfterFunc(time.Second*time.Duration(ElmExpirTime-time.Now().Unix()-60), this.refreshAccessToken)
		}
	}
}

func (this *ELM) getAccessToken() (string, error) {
	this.refreshAccessToken()
	if elmAccessToken, ok := DB.Get("ElmAccessToken").(string); ok {
		return elmAccessToken, nil
	}
	return "", errors.New("获取token出错")
}

func (this *ELM) createStore(info *CreateStore) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/chain_store"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) refreshAccessToken() {
	if elmExpirTime, ok := DB.Get("ElmExpirTime").(float64); ok {
		ElmExpirTime := int64(elmExpirTime)
		if (ElmExpirTime - time.Now().Unix()) > 60 {
			return
		}
	}
	data := make(map[string]interface{})
	data["app_id"] = appId
	data["salt"] = this.creatSalt()
	data["signature"] = this.creatAccessTokenSign(data)
	hostUrl := apiUrl + "/anubis-webapi/get_access_token"
	hostUrl = this.createRequestUrl(hostUrl, data)
	err := this.httpGet(hostUrl)
	if err != nil {
		return
	}
	if this.Code == "200" {
		res := make(map[string]interface{})
		v, ok := this.Data.(map[string]interface{})
		if ok {
			res = v
		}
		if access_token, ok := res["access_token"]; ok {
			expire_time, ok := res["expire_time"].(float64)
			if !ok {
				return
			}
			dbExpire_time := int64(expire_time)/1000 - time.Now().Unix()
			DB.Set("ElmAccessToken", fmt.Sprintf("%s", access_token), time.Second*time.Duration(dbExpire_time))
			DB.Set("ElmExpirTime", int64(expire_time)/1000, time.Second*time.Duration(dbExpire_time))
			time.AfterFunc(time.Second*time.Duration(dbExpire_time-60), this.refreshAccessToken)
		}
	}
}

func (this *ELM) accessToken() (string, error) {
	this.refreshAccessToken()
	if elmAccessToken, ok := DB.Get("ElmAccessToken").(string); ok {
		return elmAccessToken, nil
	}
	return "", errors.New("获取token失败")
}

func (this *ELM) queryStore(info *QueryStore) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/chain_store/query"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) complaintOrder(info *ComplaintOrder) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/order/complaint"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) queryOrder(info *QueryOrder) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/order/query"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) cancelOrder(info *CancelOrder) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/order/cancel"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) createOrder(info *CreateOrder) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/order"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) carrier(info *Carrier) (err error) {
	hostUrl := apiUrl + "/anubis-webapi/v2/order/carrier"
	orderInfo, err := json.Marshal(info)
	if err != nil {
		return err
	}
	err = this.Send(hostUrl, orderInfo)
	return err
}

func (this *ELM) Send(hostUrl string, info []byte) (err error) {
	requestData := make(map[string]interface{})
	requestData["app_id"] = appId
	requestData["salt"] = this.creatSalt()
	requestData["data"] = string(info)
	signature, err := this.createUserSign(requestData)
	if err != nil {
		return err
	}
	requestData["data"] = url.QueryEscape(string(info))
	requestData["signature"] = signature
	reqData, err := json.Marshal(requestData)
	err = this.httpPost(hostUrl, reqData)
	if err != nil {
		return err
	}
	if this.Code == "200" {
		return nil
	}
	return errors.New(this.Msg)
}

func (this *ELM) createRequestUrl(hostUrl string, params map[string]interface{}) string {
	param := ""
	for k, v := range params {
		param += k + "=" + fmt.Sprintf("%v", v) + "&"
	}
	if len(param) > 0 {
		param = strings.TrimRight(param, "&")
		return hostUrl + "?" + param
	}
	return hostUrl
}

func (this *ELM) verifSign(notifyInfo *Notify) (err error) {
	data := make(map[string]interface{})
	data["app_id"] = appId
	notifyData, err := url.QueryUnescape(notifyInfo.Data)
	if err != nil {
		return err
	}
	data["data"] = notifyData
	data["salt"] = notifyInfo.Salt
	sign, err := this.createUserSign(data)
	if err != nil {
		return err
	}
	if notifyInfo.Signature == sign {
		return nil
	}
	return errors.New("签名验证不通过")
}

func (this *ELM) createUserSign(mReq map[string]interface{}) (string, error) {
	if _, ok := mReq["app_id"]; !ok {
		return "", errors.New("缺少app_id")
	}
	if _, ok := mReq["data"]; !ok {
		return "", errors.New("数据不正确")
	}
	if _, ok := mReq["salt"]; !ok {
		return "", errors.New("缺少salt")
	}
	accessToken, err := this.getAccessToken()
	if err != nil {
		return "", err
	}
	//mReq["access_token"] = accessToken
	mReq["data"] = url.QueryEscape(fmt.Sprintf("%s", mReq["data"]))
	sorted_keys := make([]string, 0)
	for k, _ := range mReq {
		sorted_keys = append(sorted_keys, k)
	}
	sort.Strings(sorted_keys)
	var signStrings string
	for _, k := range sorted_keys {
		value := fmt.Sprintf("%v", mReq[k])
		if value != "" {
			if k == "app_id" {
				signStrings += k + "=" + value + "&access_token=" + accessToken + "&"
			} else {
				signStrings += k + "=" + value + "&"
			}

		}
	}
	if len(signStrings) > 0 {
		signStrings = strings.TrimRight(signStrings, "&")
	}
	return this.md5(signStrings), nil
}

func (this *ELM) creatAccessTokenSign(mReq map[string]interface{}) string {
	mReq["secret_key"] = secretKey
	sorted_keys := make([]string, 0)
	for k, _ := range mReq {
		sorted_keys = append(sorted_keys, k)
	}
	sort.Strings(sorted_keys)
	var signStrings string
	for _, k := range sorted_keys {
		value := fmt.Sprintf("%v", mReq[k])
		if value != "" {
			signStrings += k + "=" + value + "&"
		}
	}
	if len(signStrings) > 0 {
		signStrings = strings.TrimRight(signStrings, "&")
		signStrings = url.QueryEscape(signStrings)
	}
	return this.md5(signStrings)
}

/**
生成1000-9999内随机数
*/
func (this *ELM) creatSalt() int {
	rand.Seed(time.Now().UTC().UnixNano())
	return 1000 + rand.Intn(9999-1000)
}

func (this *ELM) httpPost(url string, body []byte) error {
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("http.Status: %s", httpResp.Status))
	}
	err = json.NewDecoder(httpResp.Body).Decode(this)
	if err != nil {
		return err
	}
	return nil
}

func (this *ELM) httpGet(url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("http.Status: %s", httpResp.Status))
	}
	err = json.NewDecoder(httpResp.Body).Decode(this)
	if err != nil {
		return err
	}
	return nil
}

func (this *ELM) md5(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	rs := hex.EncodeToString(h.Sum(nil))
	return rs
}
