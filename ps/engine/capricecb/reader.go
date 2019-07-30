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

package capricecb

import (
	"context"
	"errors"
	"fmt"
	"github.com/tiglabs/baudengine/ps/engine"
	"github.com/tiglabs/baudengine/ps/engine/aggregations"
	"github.com/tiglabs/baudengine/ps/engine/capricecb/querycb"
	"github.com/tiglabs/baudengine/proto"
	"github.com/tiglabs/baudengine/proto/request"
	"github.com/tiglabs/baudengine/proto/response"
	"github.com/tiglabs/baudengine/util/baudlog"
	"github.com/tiglabs/caprice"
	capriceDocument "github.com/tiglabs/caprice/document"
	"github.com/tiglabs/caprice/index"
	"github.com/tiglabs/caprice/search/highlight/format/html"
	fragmenterSimple "github.com/tiglabs/caprice/search/highlight/fragmenter/simple"
	highlighterSimple "github.com/tiglabs/caprice/search/highlight/highlighter/simple"
	"github.com/tiglabs/caprice/search/query"
	"github.com/tiglabs/log"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
)

var _ engine.Reader = &readerImpl{}

type readerImpl struct {
	*capriceEngine
	indexReaderConfig *caprice.IndexReaderConfig
}


func (reader *readerImpl) GetInternalID(ctx context.Context, docID string) (uint64, error) {
	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return 0, err
	}
	defer baudlog.CloseIfNotNil(indexReader)

	result, err := indexReader.Search(ctx, caprice.NewSearchRequest(query.NewTermQuery("_id", index.NewStringTerm(docID))))

	if err != nil {
		return 0, err
	}

	if len(result.Hits) == 0 {
		return 0, pkg.ErrDocumentNotExist
	}

	return uint64(result.Hits[0].IndexInternalID), nil
}

func (reader *readerImpl) GetDoc(ctx context.Context, docID string) *response.DocResult {
	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return response.NewErrDocResult(docID, err)
	}
	defer baudlog.CloseIfNotNil(indexReader)

	result, err := indexReader.Search(ctx, caprice.NewSearchRequest(query.NewTermQuery("_id", index.NewStringTerm(docID))))

	if err != nil {
		return response.NewErrDocResult(docID, err)
	}

	if len(result.Hits) == 0 {
		return response.NewNotFoundDocResult(docID)
	}

	match := result.Hits[0]

	document, err := indexReader.Document(match.IndexInternalID)
	if err != nil {
		return response.NewErrDocResult(docID, err)
	}

	return reader.Document2DocResult(match, document)
}

func (reader *readerImpl) GetDocs(ctx context.Context, docIDs []string) []*response.DocResult {
	result := make([]*response.DocResult, len(docIDs))

	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		for i, id := range docIDs {
			result[i] = response.NewErrDocResult(id, err)
		}
		return result
	}

	defer baudlog.CloseIfNotNil(indexReader)

	sr, err := indexReader.Search(ctx, caprice.NewSearchRequest(query.NewDocIDQuery(docIDs)))

	if err != nil {
		for i, id := range docIDs {
			result[i] = response.NewErrDocResult(id, err)
		}
		return result
	}

	idMap := make(map[string]int, len(docIDs))
	for i, id := range docIDs {
		idMap[id] = i
	}

	success := 0
	for _, hit := range sr.Hits {
		document, err := indexReader.Document(hit.IndexInternalID)
		if err != nil {
			log.Error("read doc internalId:%d err:[%s] ", hit.IndexInternalID, err.Error())
			continue
		}
		if index, found := idMap[document.DocID()]; found {
			result[index] = reader.Document2DocResult(hit, document)
			success++
		}
	}

	if success == len(result) {
		return result
	}

	for i, r := range result {
		if r == nil {
			result[i] = response.NewNotFoundDocResult(docIDs[i])
		}
	}

	return result
}

func (reader *readerImpl) MSearch(ctx context.Context, request *request.SearchRequest) response.SearchResponses {
	panic("implement me")
}

func (reader *readerImpl) Search(ctx context.Context, req *request.SearchRequest) *response.SearchResponse {
	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return response.NewSearchResponseErr(err)
	}
	defer baudlog.CloseIfNotNil(indexReader)

	q, err := querycb.NewQueryBuilder(reader.indexMapping).ParseQuery(req.Query)
	if err != nil {
		return response.NewSearchResponseErr(baudlog.LogErrAndReturn(fmt.Errorf("parse query has err:[%s] query:[%s]", err.Error(), string(req.Query))))
	}

	aggs, err := aggregations.ParseAggs(req.Aggs)
	if err != nil {
		return response.NewSearchResponseErr(baudlog.LogErrAndReturn(fmt.Errorf("parse agg has err:[%s] aggs:[%s]", err.Error(), string(req.Aggs))))
	}

	order, err := req.SortOrder()
	if err != nil {
		return response.NewSearchResponseErr(baudlog.LogErrAndReturn(fmt.Errorf("sort has err:[%s] sort:[%s]", err.Error(), string(req.Sort))))
	}

	reqs := &caprice.SearchRequest{
		Query:       q,
		Size:        *req.Size,
		From:        req.From,
		Explain:     req.Explain,
		Sort:        order,
		MinScore:    req.MinScore,
		Aggregators: aggs,
	}
	if req.Highlight != nil {
		reqs.IncludeLocations = true
	}

	result, err := indexReader.Search(ctx, reqs)
	if err != nil {
		return response.NewSearchResponseErr(err)
	}

	hits := make(response.Hits, 0, int(math.Min(1000, float64(*req.Size))))

	var highlighter *highlighterSimple.Highlighter
	if req.Highlight != nil {
		fragmenter := fragmenterSimple.NewFragmenter(req.Highlight.FragmentSize)
		if len(req.Highlight.PreTags) == 0 || len(req.Highlight.PostTags) == 0 {
			return response.NewSearchResponseErr(errors.New("highlight syntax error"))
		}
		formatter := html.NewFragmentFormatter(req.Highlight.PreTags[0], req.Highlight.PostTags[0])
		highlighter = highlighterSimple.NewHighlighter(fragmenter, formatter, highlighterSimple.DefaultSeparator)
	}

	for _, hit := range result.Hits {

		document, err := indexReader.Document(hit.IndexInternalID)
		if err != nil {
			return response.NewSearchResponseErr(err)
		}
		docResult := reader.Document2DocResult(hit, document)

		if req.Highlight != nil {
			var fragments = make(response.HighlightResult)
			for highField := range req.Highlight.Fields {
				fields, err := ReadFieldValue(reader.indexMapping, document, highField)
				if err != nil {
					log.Error("Fail to read field[%v] value for highlight", highField)
					continue
				}

				doc := capriceDocument.NewDocument(document.DocID())
				for _, field := range fields {
					if field != nil && field.Value.Text != "" {
						doc.AddField(capriceDocument.NewTextField(highField, []byte(field.Value.Text)))
					}
				}
				highFragments := highlighter.BestFragmentsInField(hit, doc, highField, 5)
				if len(highFragments) != 0 {
					fragments[highField] = highFragments
				}
			}
			docResult.Highlight = fragments
		}

		hits = append(hits, docResult)
	}
	return SearchResult2SearchResponse(result, hits)
}

func (reader *readerImpl) StreamSearch(ctx context.Context, req *request.SearchRequest, resultChan chan *response.DocResult) error {
	if ctx == nil {
		return fmt.Errorf("context must not nil")
	}

	if resultChan == nil {
		return fmt.Errorf("resultChan must not nil")
	}

	defer func() {
		close(resultChan)
	}()

	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return err
	}
	defer baudlog.CloseIfNotNil(indexReader)

	q, err := querycb.NewQueryBuilder(reader.indexMapping).ParseQuery(req.Query)
	if err != nil {
		return baudlog.LogErrAndReturn(fmt.Errorf("parse query has err:[%s] query:[%s]", err.Error(), string(req.Query)))
	}

	reqs := &caprice.StreamSearchRequest{
		Query:    q,
		MinScore: req.MinScore,
	}

	if req.Highlight != nil {
		reqs.IncludeLocations = true
	}

	result, err := indexReader.StreamSearch(ctx, reqs)
	if err != nil {
		return err
	}

	var highlighter *highlighterSimple.Highlighter
	if req.Highlight != nil {
		fragmenter := fragmenterSimple.NewFragmenter(req.Highlight.FragmentSize)
		if len(req.Highlight.PreTags) == 0 || len(req.Highlight.PostTags) == 0 {
			return errors.New("highlight syntax error")
		}
		formatter := html.NewFragmentFormatter(req.Highlight.PreTags[0], req.Highlight.PostTags[0])
		highlighter = highlighterSimple.NewHighlighter(fragmenter, formatter, highlighterSimple.DefaultSeparator)
	}

	for {

		hit, err := result.Next()
		if err != nil {
			return err
		}

		if hit == nil {
			break
		}

		document, err := indexReader.Document(hit.IndexInternalID)
		if err != nil {
			return err
		}
		docResult := reader.Document2DocResult(hit, document)

		if req.Highlight != nil {
			var fragments = make(response.HighlightResult)
			for highField := range req.Highlight.Fields {
				fields, err := ReadFieldValue(reader.indexMapping, document, highField)
				if err != nil {
					log.Error("Fail to read field[%v] value for highlight", highField)
					continue
				}

				doc := capriceDocument.NewDocument(document.DocID())
				for _, field := range fields {
					if field != nil && field.Value.Text != "" {
						doc.AddField(capriceDocument.NewTextField(highField, []byte(field.Value.Text)))
					}
				}
				highFragments := highlighter.BestFragmentsInField(hit, doc, highField, 5)
				if len(highFragments) != 0 {
					fragments[highField] = highFragments
				}
			}
			docResult.Highlight = fragments
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resultChan <- docResult
	}

	return nil
}

func (reader *readerImpl) ReadSN(ctx context.Context) (int64, error) {
	reader.lock.RLock()
	defer reader.lock.RUnlock()
	fileName := filepath.Join(reader.indexConfig.Path, indexSn)
	b, err := ioutil.ReadFile(fileName)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		} else {
			return 0, err
		}
	}
	sn, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, err
	}
	return sn, nil
}

func (reader *readerImpl) DocCount(ctx context.Context) (uint64, error) {
	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return 0, err
	}
	defer baudlog.CloseIfNotNil(indexReader)
	return indexReader.DocCount()
}

func (reader *readerImpl) Capacity(ctx context.Context) (int64, error) {
	indexReader, err := reader.index.NewIndexReader(reader.indexReaderConfig)
	if err != nil {
		return 0, err
	}
	defer baudlog.CloseIfNotNil(indexReader)
	return indexReader.Capacity()
}
