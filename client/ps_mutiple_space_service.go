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

package client

import (
	"fmt"
	"github.com/spf13/cast"
	"github.com/tiglabs/baudengine/proto/request"
	"github.com/tiglabs/baudengine/proto/response"
	"sync"
)

type multipleSpaceSender struct {
	senders []*spaceSender
}

func (this *multipleSpaceSender) MSearch(req *request.SearchRequest) (result response.SearchResponses) {
	var wg sync.WaitGroup
	respChain := make(chan response.SearchResponses, len(this.senders))

	for _, s := range this.senders {
		wg.Add(1)
		go func(par *spaceSender) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Println(r)
					respChain <- response.SearchResponses{newSearchResponseWithError(s.db, s.space, 0, fmt.Errorf(cast.ToString(r)))}
				}
			}()
			respChain <- s.MSearch(req)
		}(s)
	}

	wg.Wait()
	close(respChain)




	for r := range respChain {
		if result == nil {
			result = r
			continue
		}

		var err error

		if len(result) < len(r){
			err = mergeResultArr(r , result,req)
			result = r
		}else{
			err = mergeResultArr(result,r,req)
		}

		if err != nil {
			return response.SearchResponses{newSearchResponseWithError(this.senders[0].db, this.senders[0].space, 0, err)}
		}
	}
	return result
}

func mergeResultArr(dest response.SearchResponses, src response.SearchResponses, req *request.SearchRequest) error {

	sortOrder, err := req.SortOrder()
	if err != nil {
		return fmt.Errorf("sort err [%s]", string(req.Sort))
	}

	if len(dest)==len(src){
		for index := range dest{
			err := dest[index].Merge(src[index], sortOrder, req.From, *req.Size)
			if err != nil {
				return fmt.Errorf("merge err [%s]")
			}
		}
	}else{
		for index := range dest{
			err := dest[index].Merge(src[0], sortOrder, req.From, *req.Size)
			if err != nil {
				return fmt.Errorf("merge err [%s]")
			}
		}
	}

	return nil

}

func (this *multipleSpaceSender) Search(req *request.SearchRequest) (result *response.SearchResponse) {
	var wg sync.WaitGroup
	respChain := make(chan *response.SearchResponse, len(this.senders))

	for _, s := range this.senders {
		wg.Add(1)
		go func(par *spaceSender) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Println(r)
					respChain <- newSearchResponseWithError(s.db, s.space, 0, fmt.Errorf(cast.ToString(r)))
				}
			}()
			respChain <- s.Search(req)
		}(s)
	}

	wg.Wait()
	close(respChain)

	sortOrder, err := req.SortOrder()
	if err != nil {
		return newSearchResponseWithError(this.senders[0].db, this.senders[0].space, 0, err)
	}

	for r := range respChain {
		if result == nil {
			result = r
			continue
		}
		err := result.Merge(r, sortOrder, req.From, *req.Size)
		if err != nil {
			return newSearchResponseWithError(this.senders[0].db, this.senders[0].space, 0, err)
		}
	}
	return result
}

func (this *multipleSpaceSender) StreamSearch(req *request.SearchRequest) (dsr *response.DocStreamResult) {
	ctx := req.Context().GetContext()
	dsr = response.NewDocStreamResult(ctx)

	go func() {
		defer func() {
			dsr.AddDoc(nil)
		}()
		for _, s := range this.senders {
			spaceDsr := s.StreamSearch(req)
			for {
				doc, err := spaceDsr.Next()
				if err != nil {
					dsr.AddErr(err)
					return
				}
				if doc == nil {
					break
				}
				dsr.AddDoc(doc)
			}
		}
	}()
	return dsr
}
