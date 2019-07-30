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

package heapsort

import "container/heap"

// this is a max or min heap implementation for slice
type internalHeap []Comparator

func (h internalHeap) Len() int           { return len(h) }
func (h internalHeap) Less(i, j int) bool { return h[i].LessThan(h[j]) }
func (h internalHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *internalHeap) Push(x interface{}) {
    *h = append(*h, x.(Comparator))
}

func (h *internalHeap) Pop() interface{} {
    old := *h
    n := len(old)
    x := old[n-1]
    *h = old[0 : n-1]
    return x
}

// implement normal less that while using min heap
// implement reversed compare while using max heap
type Comparator interface {
    LessThan(other Comparator) bool
}

type Heap interface {
    Push(e Comparator)

    Pop() Comparator

    Top() Comparator

    TopShuffle()

    Size() int
}

type HeapImpl struct {
    h *internalHeap
}

func NewHeap() Heap {
    impl := new(HeapImpl)
    impl.h = new(internalHeap)
    heap.Init(impl.h)
    return impl
}

func (mh *HeapImpl) Push(e Comparator) {
    heap.Push(mh.h, e)
}

func (mh *HeapImpl) Pop() Comparator {
    if len(*mh.h) <= 0 {
        return nil
    }

    e := heap.Pop(mh.h)
    if e != nil {
        return e.(Comparator)
    } else {
        return nil
    }
}

func (mh *HeapImpl) Top() Comparator {
    if len(*mh.h) <= 0 {
        return nil
    }
    return (*mh.h)[0].(Comparator)
}

func (mh *HeapImpl) TopShuffle() {
    if len(*mh.h) <= 0 {
        return
    }
    heap.Fix(mh.h, 0)
}

func (mh *HeapImpl) Size() int {
    return mh.h.Len()
}

