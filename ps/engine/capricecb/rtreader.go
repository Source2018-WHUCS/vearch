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
    "github.com/tiglabs/baudengine/ps/engine"
    "github.com/tiglabs/baudengine/proto/response"
)

var _ engine.RTReader = &rtReaderImpl{}

type rtReaderImpl struct {
	writer *writerImpl
}

// NewRTReader return a real time document reader
func NewRTReader(writer *writerImpl) *rtReaderImpl {
	return &rtReaderImpl{writer:writer}
}

//   RTReadDoc(ctx context.Context, docID string) *entity.DocResult
func (rtReader rtReaderImpl)RTReadDoc(ctx context.Context, docID string) *response.DocResult {
	return rtReader.writer.getDocForUpdate(ctx, docID)
}
