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

package master

import "C"
import (
	"context"
	"fmt"
	"github.com/vearch/vearch/util/server/vearchhttp"
	"github.com/vearch/vearch/util/uuid"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/vearch/vearch/proto"
	"github.com/vearch/vearch/util/netutil"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"github.com/vearch/vearch/config"
	"github.com/vearch/vearch/proto/entity"
	"github.com/vearch/vearch/util"
	"github.com/vearch/vearch/util/ginutil"
	"github.com/vearch/vearch/util/log"
)

type monitorApi struct {
	router         *gin.Engine
	monitorService *monitorService
	dh             *vearchhttp.BaseHandler
}

func ExportToMonitorHandler(router *gin.Engine, monitorService *monitorService) {

	dh := vearchhttp.NewBaseHandler(30)

	c := &monitorApi{router: router, monitorService: monitorService, dh: dh}

	//cluster handler
	router.Handle(http.MethodGet, "/_cluster/health", dh.PaincHandler, dh.TimeOutHandler, c.auth, c.health, dh.TimeOutEndHandler)
	router.Handle(http.MethodGet, "/_cluster/stats", dh.PaincHandler, dh.TimeOutHandler, c.auth, c.stats, dh.TimeOutEndHandler)
	//metrics
	router.Handle(http.MethodPost, "/metrics", dh.PaincHandler, dh.TimeOutHandler, c.auth, c.metrics, dh.TimeOutEndHandler)
}

//got every partition servers system info
func (this *monitorApi) stats(c *gin.Context) {
	ctx, _ := c.Get(vearchhttp.Ctx)
	list, err := this.monitorService.statsService(ctx.(context.Context))
	if err != nil {
		ginutil.NewAutoMehtodName(c).SendJsonHttpReplyError(err)
		return
	}
	ginutil.NewAutoMehtodName(c).SendJson(list)
}

//cluster health in partition level
func (this *monitorApi) health(c *gin.Context) {

	ctx, _ := c.Get(vearchhttp.Ctx)

	dbName := c.Query("db")
	spaceName := c.Query("space")

	result, err := this.monitorService.partitionInfo(ctx.(context.Context), dbName, spaceName)
	if err != nil {
		ginutil.NewAutoMehtodName(c).SendJsonHttpReplyError(err)
		return
	}

	ginutil.NewAutoMehtodName(c).SendJson(result)
}

func (this *monitorApi) _auth(ctx context.Context, c *gin.Context) error {

	if config.Conf().Global.SkipAuth {
		return nil
	}

	headerData := c.GetHeader(headerAuthKey)

	if headerData == "" {
		return pkg.CodeErr(pkg.ERRCODE_AUTHENTICATION_FAILED)
	}

	username, password, err := util.AuthDecrypt(headerData)
	if err != nil {
		return err
	}

	if username != "root" || password != config.Conf().Global.Signkey {
		return pkg.CodeErr(pkg.ERRCODE_AUTHENTICATION_FAILED)
	}

	return nil
}

func (this *monitorApi) metrics(c *gin.Context) {
	//ctx, _ := c.Get(vearchhttp.Ctx)

}