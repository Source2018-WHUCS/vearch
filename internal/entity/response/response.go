// Copyright 2019 The Vearch Authors.
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

package response

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/vearch/vearch/internal/entity/errors"
	"github.com/vearch/vearch/internal/pkg/log"
	"github.com/vearch/vearch/internal/pkg/vjson"
)

type Response struct {
	ginContext *gin.Context
	httpStatus int64
}

// http protocal
type HttpReply struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

func New(ginContext *gin.Context) *Response {
	return &Response{
		ginContext: ginContext,
		httpStatus: http.StatusOK,
	}
}

/*
default status is 200
*/
func (r *Response) SetHttpStatus(httpStatus int64) *Response {
	r.httpStatus = httpStatus
	return r
}

func (r *Response) SendJson(data interface{}) {
	reply, err := vjson.Marshal(data)
	if err != nil {
		r.JsonError(err)
		return
	}
	r.ginContext.Data(int(r.httpStatus), "application/json", reply)
}

func (r *Response) SendJsonBytes(bytes []byte) {
	r.ginContext.Header("Content-Type", "application/json; charset=UTF-8")
	r.ginContext.Header("Content-Length", strconv.Itoa(len(bytes)))
	r.ginContext.Writer.WriteHeader(int(r.httpStatus))

	if _, err := r.ginContext.Writer.Write(bytes); err != nil {
		log.Errorf("fail to write http reply, err:[%v], body:[%s] len[%d]", err, string(bytes), len(bytes))
	}
}

func (r *Response) JsonSuccess(data interface{}) {
	httpReply := &HttpReply{
		Code: http.StatusOK,
		Msg:  "",
		Data: data,
	}
	r.SetHttpStatus(int64(httpReply.Code))
	r.SendJson(httpReply)
}

func (r *Response) SuccessDelete() {
	httpReply := &HttpReply{
		Code: http.StatusNoContent,
		Msg:  "",
	}
	r.SetHttpStatus(int64(httpReply.Code))
	r.SendJson(httpReply)
}

func (r *Response) JsonError(err error) {
	if err == nil {
		err = fmt.Errorf("")
	}

	code := http.StatusInternalServerError
	if _, ok := err.(errors.ErrBadRequest); ok {
		code = http.StatusBadRequest
	} else if _, ok := err.(errors.ErrUnprocessable); ok {
		code = http.StatusUnprocessableEntity
	} else if _, ok := err.(errors.ErrNotFound); ok {
		code = http.StatusNotFound
	} else if _, ok := err.(errors.ErrInternal); ok {
		code = http.StatusInternalServerError
	}

	httpReply := &HttpReply{
		Code: code,
		Msg:  err.Error(),
	}
	r.SetHttpStatus(int64(httpReply.Code))
	r.SendJson(httpReply)
}
