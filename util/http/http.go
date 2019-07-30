/*
Copyright 2018 Tiger

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreedto in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

created by lvxin for project baudengine at 18-10-29 下午3:00
*/
package http

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"
	"fmt"
	"strings"
	"net/url"
)

var (
	ContentTypeJson = "application/json; charset=utf-8"
	ContentTypeForm = "application/x-www-form-urlencoded"
)

// PostJSON - send an http post json Request.
func PostJSON(url string, data interface{}, deadline, dialTimeout time.Duration) ([]byte, int, error) {
	buf, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}

	return Request(http.MethodPost, url, bytes.NewBuffer(buf), deadline, dialTimeout, map[string]string{"Content-Type": ContentTypeJson})
}

// PostForm - sen an http post form Request
func PostForm(url string, data []byte, deadline, dialTimeout time.Duration) ([]byte, int, error) {
	return Request(http.MethodPost, url, bytes.NewBuffer(data), deadline, dialTimeout, map[string]string{"Content-Type": ContentTypeForm})
}

// SimpleGet - send an http get Request
func SimpleGet(url string, deadline, dialTimeout time.Duration) ([]byte, int, error) {
	return Request(http.MethodGet, url, bytes.NewBuffer(nil), deadline, dialTimeout, nil)
}

// SimpleDelete - send an simple http delete Request
func SimpleDelete(url string, deadline, dialTimeout time.Duration) ([]byte, int, error) {
	return Request(http.MethodDelete, url, bytes.NewBuffer(nil), deadline, dialTimeout, nil)
}

// SimpleDelete - send an simple http delete Request
func HttpDelete(endpoint string,urlParams url.Values, formData url.Values,deadline, dialTimeout time.Duration) ([]byte, int, error) {
	httpUrl:=endpoint
	params := url.Values{}
	// This adds the optional url parameters settings to the URL
	if urlParams != nil {
		for key := range urlParams {
			params.Add(key, urlParams.Get(key))
		}
		httpUrl = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}
	// If supplied, encode the formdata to include in the body
	body := strings.NewReader(formData.Encode())
	return Request(http.MethodDelete, httpUrl, body, deadline, dialTimeout, nil)
}
// SimplePut - send an simple http put Request
func SimplePut(url string, deadline, dialTimeout time.Duration) ([]byte, int, error) {
	return Request(http.MethodPut, url, bytes.NewBuffer(nil), deadline, dialTimeout, nil)
}

// CookiesGet - send an http get Request, response cookies
func CookiesGet(url string, deadline, dialTimeout time.Duration, header map[string]string) ([]byte, []*http.Cookie, error) {
	return RequestCookies(http.MethodGet, url, bytes.NewBuffer(nil), deadline, dialTimeout, header, nil)
}

// GetRequestWithBasicAuth - get request with Basic Auth
func GetRequestWithBasicAuth(url string, deadline, dialTimeout time.Duration, username, password string) ([]byte, int, error) {
	return BasicAuthRequest(http.MethodGet, url, bytes.NewBuffer(nil), deadline, dialTimeout, nil, username, password)
}

// BasicAuthRequest - send an http Request
func BasicAuthRequest(method, url string, body io.Reader, deadline, dialTimeout time.Duration, header map[string]string, username, password string) ([]byte, int, error) {
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				deadline := time.Now().Add(deadline)
				c, err := net.DialTimeout(netw, addr, dialTimeout)
				if err != nil {
					return nil, err
				}
				c.SetDeadline(deadline)
				return c, nil
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, err
	}

	if header != nil {
		for key, value := range header {
			req.Header.Set(key, value)
		}
	}

	req.SetBasicAuth(username, password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

// Request - send an http Request
func Request(method, url string, body io.Reader, deadline, dialTimeout time.Duration, header map[string]string) ([]byte, int, error) {
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				deadline := time.Now().Add(deadline)
				c, err := net.DialTimeout(netw, addr, dialTimeout)
				if err != nil {
					return nil, err
				}
				c.SetDeadline(deadline)
				return c, nil
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, 0, err
	}

	if header != nil {
		for key, value := range header {
			req.Header.Set(key, value)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

// RequestCookies - send an http Request with cookies
func RequestCookies(method, url string, body io.Reader, deadline, dialTimeout time.Duration, header map[string]string, cookies []*http.Cookie) ([]byte, []*http.Cookie, error) {
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				deadline := time.Now().Add(deadline)
				c, err := net.DialTimeout(netw, addr, dialTimeout)
				if err != nil {
					return nil, err
				}
				c.SetDeadline(deadline)
				return c, nil
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, nil, err
	}

	if header != nil {
		for key, value := range header {
			req.Header.Set(key, value)
		}
	}

	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return data, resp.Cookies(), nil
}

// HttpResponse - htt Response
type HttpResponse struct {
	Code    int         `json:"Code"`
	Message string      `json:"Message"`
	Data    interface{} `json:"Data,omitempty"`
}

// NewHttpResponse -
func NewHttpResponse() *HttpResponse {
	return &HttpResponse{
		Code:    0,
		Message: "success",
	}
}

// Response - write data to resp
func (h *HttpResponse) Response(resp http.ResponseWriter) {
	resp.WriteHeader(http.StatusOK)
	data, _ := json.Marshal(h)
	resp.Write(data)
}

// ResponseWithErr - write data to resp with error
func (h *HttpResponse) ResponseWithErr(resp http.ResponseWriter, err error) {
	resp.WriteHeader(http.StatusOK)
	if err != nil {
		h.Error(err)
	}

	data, _ := json.Marshal(h)
	resp.Write(data)
}

// set data
func (h *HttpResponse) SetData(data interface{}) *HttpResponse {
	h.Data = data
	return h
}

// Error - set Error
func (h *HttpResponse) Error(err error) {
	h.Code = 1
	h.Message = err.Error()
}

// RespData user for bootstrap
type RespData struct {
	Total int64       `json:"total"`
	Rows  interface{} `json:"rows"`
}
