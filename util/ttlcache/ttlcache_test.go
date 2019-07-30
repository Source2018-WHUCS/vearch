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

package ttlcache

import (
    "testing"
    "time"
    "github.com/tiglabs/baudengine/util/assert"
    "strconv"
)

func TestNormalTTLCache(t *testing.T) {
    type TN struct {
        v int
    }

    cache := NewTTLCache(5 * time.Second)
    for i := 1; i <= 10; i++ {
        cache.Put("key"+strconv.Itoa(i), TN{v: 1000 + i})
    }

    tn5, ok5 := cache.Get("key5")
    assert.True(t, ok5)
    assert.Equal(t, tn5.(TN).v, 1005, "get error")

    for i := 1; i <= 10; i++ {
        tn, ok := cache.Get("key" + strconv.Itoa(i))
        assert.True(t, ok)
        assert.Equal(t, tn.(TN).v, 1000 + i, "get error")
    }

    values1 := cache.Values()
    assert.NotNil(t, values1)
    assert.Equal(t, len(values1), 10, "values error")

    cache.Delete("key5")
    tn5, ok5 = cache.Get("key5")
    assert.False(t, ok5)
    assert.Nil(t, tn5)

    time.Sleep(6 * time.Second)
    values2 := cache.Values()
    assert.Nil(t, values2)

}

func TestAfterClearTTLCache(t *testing.T) {
    type TN struct {
        v int
    }
    assert.Equal(t, TTLCACHE_CLEAR_INTERVAL, 10 * time.Second, "clear interval error")

    cache := NewTTLCache(11 * time.Second)
    for i := 1; i <= 10; i++ {
        cache.Put("key"+strconv.Itoa(i), TN{v: 1000 + i})
    }
    assert.Equal(t, 10, cache.Size(), "size error")

    time.Sleep(12 * time.Second)
    assert.Equal(t, 0, cache.Size(), "clear error")
}
