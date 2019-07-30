// Copyright 2018 The ChuBao Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package testutil

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/entity"
    "github.com/tiglabs/baudengine/proto/response"
    "github.com/tiglabs/baudengine/util"
	"github.com/tiglabs/baudengine/util/cbjson"
	"github.com/tiglabs/baudengine/util/metrics/mserver"
	"github.com/tiglabs/baudengine/util/netutil"
	"github.com/tiglabs/log"
	"net/http"
	"net/url"
)

func NewCBClient(routerAddr, masterAddr string, dbName, spaceName string) *CBClient {
	return &CBClient{RouterAddr: routerAddr, MasterAddr: masterAddr, DbName: dbName, SpaceName: spaceName}
}

type CBClient struct {
	RouterAddr string
	MasterAddr string
	DbName     string
	SpaceName  string
}

func NewResponse(resp []byte) *Response {
	return &Response{Resp: resp, Status: 200}
}

type Response struct {
	Resp   []byte
	Status int
}

//db handler begin
func (client *CBClient) DbCreate(db *entity.DB) (*Response, error) {
	jsonData, err := cbjson.Marshal(db)

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/db/_create")
	query.SetReqBody(string(jsonData))
	query.SetHeader("Content-Type", "application/json")
	url := query.GetUrl()
	log.Info("\n test es normal create db url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal create db result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("create db err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return NewResponse(response), nil
}

func (response *Response) Check() {
	if response.Status != 200 {
		panic(string(response.Resp))
	}
}

func (client *CBClient) DbDelete(dbName string) (*Response, error) {
	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/db/" + dbName)
	query.SetContentTypeJson()
	res, err := query.Do()

	log.Info("delete db response: %s\n", string(res))
	body := Json2map(res)
	if _, ok := body["code"]; !ok {
		return nil, fmt.Errorf("invalid response format of db delete request, colum 'code' not exist")
	}
	code, err := cast.ToIntE(body["code"])
	if err != nil || code != 200 {
		return nil, fmt.Errorf("db delete failed as %s", string(res))
	}
	return &Response{Resp: res, Status: 200}, nil
}

//it will delete all space and remove db
func (client *CBClient) DbDrop(dbName string) (*Response, error) {
	db, err := client.DBGet(dbName)
	if err != nil {
		log.Warn(err.Error())
	}
	if db == nil {
		return nil, pkg.ErrMasterDbNotExists
	}

	spaces, err := client.SpaceList(dbName)
	if err != nil {
		return nil, err
	}

	for _, s := range spaces {
		if response, err := client.SpaceDelete(dbName, s.Name); err != nil {
			log.Error(err.Error())
		} else {
			log.Info(string(response.Resp))
		}
	}

	return client.DbDelete(dbName)
}

func (client *CBClient) DBGet(dbName string) (*entity.DB, error) {
	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/db/" + dbName)
	query.SetContentTypeJson()
	res, err := query.Do()

	log.Info("get db response: %s\n", string(res))
	body := Json2map(res)
	if _, ok := body["code"]; !ok {
		return nil, fmt.Errorf("invalid response format of db delete request, colum 'code' not exist")
	}
	code, err := cast.ToIntE(body["code"])
	if err != nil || code != 200 {
		return nil, fmt.Errorf("db delete failed as %s", string(res))
	}

	db := &entity.DB{}

	bytes, err := json.Marshal(body["data"])
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(bytes, db); err != nil {
		return nil, err
	}
	return db, nil
}

func (client *CBClient) DBList() ([]*entity.DB, error) {
	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/list/db")
	query.SetContentTypeJson()
	res, err := query.Do()

	log.Info("delete db response: %s\n", string(res))
	body := Json2map(res)
	if _, ok := body["code"]; !ok {
		return nil, fmt.Errorf("invalid response format of db delete request, colum 'code' not exist")
	}
	code, err := cast.ToIntE(body["code"])
	if err != nil || code != 200 {
		return nil, fmt.Errorf("db delete failed as %s", string(res))
	}

	dbList := make([]*entity.DB, 0, 3)

	if err := json.Unmarshal([]byte(cast.ToString(body["data"])), dbList); err != nil {
		return nil, err
	}
	return dbList, nil
}

//space handler begin
func (client *CBClient) SpaceCreate(dbName string, space *entity.Space) (*Response, error) {

	bytes, err := json.Marshal(space)
	if err != nil {
		return nil, err
	}

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/_create")
	query.SetReqBody(string(bytes))
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("\n test es normal create space url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal create space result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("create space err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return &Response{Resp: response, Status: 200}, nil
}

func (client *CBClient) SpaceDelete(dbName, spaceName string) (*Response, error) {
	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/" + spaceName)
	url := query.GetUrl()
	log.Info("\n test es normal delete space url %v", url)
	response, err := query.Do()
	log.Info("\n test es normal delete space result: %v", string(response))
	if err != nil {
		return nil, err
	}

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("delete space err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return NewResponse(response), nil
}

func (client *CBClient) SpaceGet(dbName , spaceName string) (*entity.Space, error) {
	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/" + spaceName)
	query.SetContentTypeJson()
	res, err := query.Do()

	body := Json2map(res)
	if _, ok := body["code"]; !ok {
		return nil, fmt.Errorf("invalid response format of db delete request, colum 'code' not exist")
	}
	code, err := cast.ToIntE(body["code"])
	if err != nil || code != 200 {
		return nil, fmt.Errorf("db delete failed as %s", string(res))
	}

	space := &entity.Space{}

	bytes, err := json.Marshal(body["data"])
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(bytes, space); err != nil {
		return nil, err
	}
	return space, nil
}

func (client *CBClient) SpaceList(dbName string) ([]*entity.Space, error) {
	form := url.Values{}
	form.Add("db", dbName)

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetQuery(form.Encode())
	query.SetUrlPath("/list/space")
	url := query.GetUrl()
	log.Info("\n test es normal list space url %v", url)
	response, err := query.Do()
	log.Info("\n test es normal list space result: %v", string(response))
	if err != nil {
		return nil, err
	}

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("list space err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	bytes, err := jsonMap.GetJsonValBytes("data")
	if err != nil {
		return nil, err
	}

	spaces := make([]*entity.Space, 0, 3)

	if err := json.Unmarshal(bytes, &spaces); err != nil {
		return nil, err
	}

	return spaces, nil
}

func (client *CBClient) SpaceChange(dbName, spaceName string, space *entity.Space) (*Response, error) {
	bytes, err := json.Marshal(space)
	if err != nil {
		return nil, err
	}

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/space/" + dbName + "/" + spaceName)
	query.SetReqBody(string(bytes))
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("\n test es normal change space url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal change space result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("change space err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return &Response{Resp: response, Status: 200}, nil
}

//user hanlder begin
func (client *CBClient) UserCreate(name, password, privilege, host, dblist string) (*Response, error) {
	form := url.Values{}
	form.Add("user_name", name)
	form.Add("user_password", password)
	form.Add("privilege", privilege)
	form.Add("user_host", host)
	form.Add("user_db_list", dblist)

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/manage/user/create")
	query.SetContentTypeForm()
	query.SetReqBody(form.Encode())
	url := query.GetUrl()
	log.Info("\n test es normal create user url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal create user result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}

	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("create user err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return NewResponse(response), nil
}

func (client *CBClient) UserList(space *entity.User) ([]*entity.User, error) {

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/manage/user/list")
	url := query.GetUrl()
	log.Info("\n test es normal list space url %v", url)
	response, err := query.Do()
	log.Info("\n test es normal list space result: %v", string(response))
	if err != nil {
		return nil, err
	}

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("list user err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	bytes, err := jsonMap.GetJsonValBytes("data")
	if err != nil {
		return nil, err
	}

	users := make([]*entity.User, 0, 3)

	if err := json.Unmarshal(bytes, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (client *CBClient) UserDelete(name string) (*Response, error) {

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/manage/user/delete?user_name=" + name)
	url := query.GetUrl()
	log.Info("\n test es normal delete user url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal delete user result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("delete user err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return NewResponse(response), nil
}

func (client *CBClient) UserAddDB(userName , dbName string) (*Response, error) {

	form := url.Values{}
	form.Add("user_name", userName)
	form.Add("user_db_list", dbName)

	address := "http://" + client.MasterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().RootName, C().RootPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/manage/user/grant/db")
	query.SetReqBody(form.Encode())

	url := query.GetUrl()
	log.Info("\n test es normal add userdb url %v", url)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal add userdb result: %v", string(response))

	jsonMap, err := cbjson.ByteToJsonMap(response)
	if err != nil {
		return nil, err
	}

	code, err := jsonMap.GetJsonValIntE("code")
	if err != nil {
		return nil, err
	}
	if code != pkg.ERRCODE_SUCCESS {
		return nil, fmt.Errorf("add userdb err: %v", jsonMap.GetJsonValStringOrDefault("msg", ""))
	}

	return NewResponse(response), nil

}

func (client *CBClient) UserDelDB(userName string, dbName string) (*Response, error) {
	panic("implement me")
}

func (client *CBClient) UserPrivilegeAdd(userName, privilege string) (*Response, error) {
	panic("implement me")
}

func (client *CBClient) UserPrivilegeDel(userName, privilege string) (*Response, error) {
	panic("implement me")
}

//document handler begin

func (client *CBClient) DocumentUpdateOrCreate(docID string, doc interface{}) (*response.DocResult, error) {
	var data string
	switch doc.(type) {
	case []byte, string:
		data = cast.ToString(doc)
	default:
		data = cbjson.ToJsonString(doc)

	}

	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID)
	query.SetReqBody(data)
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("test es normal insert doc url %s", url)
	log.Info("test es normal insert doc body %s", doc)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal insert doc result: %v", string(resp))

	jsonMap, err := cbjson.ByteToJsonMap(resp)
	if err != nil {
		return nil, err
	}

	result := jsonMap.GetJsonVal("result")
	if result == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			return nil, err
		}
		if status != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf(jsonMap.GetJsonMap("error").GetJsonValString("reason"))
		}
	}

	docResult := &response.DocResult{}
	if err := json.Unmarshal(resp, docResult); err != nil {
		return nil, err
	}

	return docResult, nil
}

func (client *CBClient) DocumentGet(docID string) (*response.DocResult, error) {
	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodGet)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID)
	url := query.GetUrl()
	log.Info("\n test es normal get doc url %v", url)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal get doc result: %v", string(resp))

	dr := &response.DocResult{}
	if err := json.Unmarshal([]byte(resp), dr); err != nil {
		return nil, err
	}

	return dr, nil
}

func (client *CBClient) DocumentCreate(docID string, doc interface{}) (*response.DocResult, error) {
	var data string
	switch doc.(type) {
	case []byte, string:
		data = cast.ToString(doc)
	default:
		data = cbjson.ToJsonString(doc)
	}

	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodPut)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID + "/_create")
	query.SetReqBody(data)
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("\n test es normal insert doc url %s", url)
	log.Info("\n test es normal insert doc body %s", data)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal insert doc result: %v", string(resp))

	jsonMap, err := cbjson.ByteToJsonMap(resp)
	if err != nil {
		return nil, err
	}

	result := jsonMap.GetJsonVal("result")
	if result == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			return nil, err
		}
		if status != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf(jsonMap.GetJsonMap("error").GetJsonValString("reason"))
		}
	}

	dr := &response.DocResult{}
	if err := json.Unmarshal([]byte(resp), dr); err != nil {
		return nil, err
	}

	return dr, nil
}


func (client *CBClient) DocumentBulk(data string) (*Response, error) {
	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/_bulk")
	query.SetReqBody(data)
	query.SetContentTypeJson()
	//url := query.GetUrl()
	log.Info("\n test es normal bulk url %s", query.GetUrl())
	log.Info("\n test es normal bulk body %s", data)
	response, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal bulk result: %v", string(response))

	return NewResponse(response), nil
}

func (client *CBClient) DocumentDelete(docID string) (*response.DocResult, error) {
	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodDelete)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID)
	url := query.GetUrl()
	log.Info("\n test es normal delete doc url %v", url)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal delete doc result: %v", string(resp))

	jsonMap, err := cbjson.ByteToJsonMap(resp)
	if err != nil {
		return nil, err
	}

	result := jsonMap.GetJsonVal("result")
	if result == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			return nil, err
		}
		if status != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf(jsonMap.GetJsonValStringOrDefault("error", ""))
		}
	}

	dr := &response.DocResult{}
	if err := json.Unmarshal([]byte(resp), dr); err != nil {
		return nil, err
	}

	return dr, nil
}

func (client *CBClient) DocumentUpdate(docID string, doc interface{}) (*response.DocResult, error) {
	var data string
	switch doc.(type) {
	case []byte, string:
		data = cast.ToString(doc)
	default:
		data = cbjson.ToJsonString(doc)
	}

	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID + "/_update")
	query.SetReqBody(data)
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("\n test es normal insert doc url %v", url)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal insert doc result: %v", string(resp))

	jsonMap, err := cbjson.ByteToJsonMap(resp)
	if err != nil {
		return nil, err
	}

	created := jsonMap.GetJsonVal("_created")
	if created == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			return nil, err
		}
		if status != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf(jsonMap.GetJsonValStringOrDefault("error", ""))
		}
	}

	dr := &response.DocResult{}
	if err := json.Unmarshal([]byte(resp), dr); err != nil {
		return nil, err
	}

	return dr, nil
}

func (client *CBClient) DocumentReplace(docID string, doc interface{}) (*response.DocResult, error) {
	var data string
	switch doc.(type) {
	case []byte, string:
		data = cast.ToString(doc)
	default:
		data = cbjson.ToJsonString(doc)
	}

	address := "http://" + client.RouterAddr
	query := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	query.SetMethod(http.MethodPost)
	query.SetAddress(address)
	query.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/" + docID)
	query.SetReqBody(data)
	query.SetContentTypeJson()
	url := query.GetUrl()
	log.Info("\n test es normal insert doc url %s", url)
	log.Info("\n test es normal insert doc body %s", data)
	resp, err := query.Do()
	if err != nil {
		return nil, err
	}
	log.Info("\n test es normal insert doc result: %v", string(resp))

	jsonMap, err := cbjson.ByteToJsonMap(resp)
	if err != nil {
		return nil, err
	}

	result := jsonMap.GetJsonVal("result")
	if result == nil {
		status, err := jsonMap.GetJsonValIntE("status")
		if err != nil {
			return nil, err
		}
		if status != pkg.ERRCODE_SUCCESS {
			return nil, fmt.Errorf(jsonMap.GetJsonMap("error").GetJsonValString("reason"))
		}
	}

	dr := &response.DocResult{}
	if err := json.Unmarshal([]byte(resp), dr); err != nil {
		return nil, err
	}

	return dr, nil
}

func (client *CBClient) Search(method, data string) (*Response, error) {
	address := "http://" + client.RouterAddr
	sender := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	sender.SetMethod(method)
	sender.SetAddress(address)
	sender.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/_search")
	sender.SetReqBody(data)
	sender.SetContentTypeJson()
	url := sender.GetUrl()
	log.Info("\n test es normal ciSearchTerm doc url %v", url)
	log.Info("\n test es normal ciSearchTerm doc reqBody %v", data)
	resp, err := sender.Do()

	if err != nil {
		return nil, err
	}
	return NewResponse(resp), nil
}

func (client *CBClient) StreamSearch(method, data string) (*http.Response, error) {
	address := "http://" + client.RouterAddr
	sender := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	sender.SetMethod(method)
	sender.SetAddress(address)
	sender.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/_stream_search")
	sender.SetReqBody(data)
	sender.SetContentTypeJson()
	url := sender.GetUrl()
	log.Info("\n test es normal ciSearchTerm doc url %v", url)
	log.Info("\n test es normal ciSearchTerm doc reqBody %v", data)
	return sender.DoResponse()
}

func (client *CBClient) SearchByParam(method string, param string, data string) (*Response, error) {

	address := "http://" + client.RouterAddr
	sender := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	sender.SetMethod(method)
	sender.SetAddress(address)
	sender.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/_search?" + param)
	sender.SetReqBody(data)
	sender.SetContentTypeJson()
	url := sender.GetUrl()
	log.Info("\n test es normal ciSearchTerm doc url %v", url)
	resp, err := sender.Do()

	if err != nil {
		return nil, err
	}
	return NewResponse(resp), nil
}

//other handler

func (client *CBClient) ServerList() ([]*entity.Server, error) {
	panic("implement me")
}

func (client *CBClient) Health() (*Response, error) {
	panic("implement me")
}

func (client *CBClient) Stats() ([]*mserver.ServerStats, error) {
	panic("implement me")
}

func (client *CBClient) Flush() (*Response, error) {
	address := "http://" + client.RouterAddr
	sender := netutil.NewQuery().SetHeader("Authorization", util.AuthEncrypt(C().UserName, C().UserPassword))
	sender.SetMethod(http.MethodPost)
	sender.SetAddress(address)
	sender.SetUrlPath("/" + client.DbName + "/" + client.SpaceName + "/_flush")
	sender.SetContentTypeJson()
	url := sender.GetUrl()
	log.Info("\n test es normal flush url %v", url)
	resp, err := sender.Do()

	if err != nil {
		return nil, err
	}
	return NewResponse(resp), nil
}
