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

import (
    "testing"
    "fmt"
    "github.com/tiglabs/baudengine/util/assert"
)

type MinElem struct {
    val int
}

func (e *MinElem) LessThan(other Comparator) bool {
    otherElem, ok := other.(*MinElem)
    if !ok {
        fmt.Printf("Can not convert to type MinElem.")
        return false
    }

    return e.val < otherElem.val
}

func TestMinHeap(t *testing.T) {
    h := NewHeap()
    for i := 0; i < 10; i+=2 {
        h.Push(&MinElem{val:i})
    }
    for i := 1; i < 10; i+=2 {
        h.Push(&MinElem{val:i})
    }

    for i := 0; i < 10; i++ {
        assert.Equal(t, h.Top().(*MinElem).val, i, "top error")
        assert.Equal(t, h.Pop().(*MinElem).val, i, "pop error")
        assert.Equal(t, h.Size(), 9 - i, "size error")
    }
    assert.Nil(t, h.Pop())

    h.Push(&MinElem{val:2})
    h.Push(&MinElem{val:1})
    assert.Equal(t, h.Top().(*MinElem).val, 1, "top error")
    h.Top().(*MinElem).val = 3
    h.TopShuffle()
    assert.Equal(t, h.Top().(*MinElem).val, 2, "top error")
}

type MaxElem struct {
    val int
}

func (e *MaxElem) LessThan(other Comparator) bool {
    otherElem, ok := other.(*MaxElem)
    if !ok {
        fmt.Printf("Can not convert type MaxElem.")
        return false
    }

    return e.val > otherElem.val
}

func TestMaxHeap(t *testing.T) {
    h := NewHeap()
    for i := 0; i < 10; i += 2 {
        h.Push(&MaxElem{val: i})
    }
    for i := 1; i < 10; i += 2 {
        h.Push(&MaxElem{val: i})
    }

    for i := 0; i < 10; i++ {
        assert.Equal(t, h.Top().(*MaxElem).val, 9-i, "top error")
        assert.Equal(t, h.Pop().(*MaxElem).val, 9-i, "pop error")
        assert.Equal(t, h.Size(), 9-i, "size error")
    }
    assert.Nil(t, h.Pop())

    h.Push(&MaxElem{val: 1})
    h.Push(&MaxElem{val: 2})
    assert.Equal(t, h.Top().(*MaxElem).val, 2, "top error")
    h.Top().(*MaxElem).val = 0
    h.TopShuffle()
    assert.Equal(t, h.Top().(*MaxElem).val, 1, "top error")
}
