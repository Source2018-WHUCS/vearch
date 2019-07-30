#include "mem_cache.h"
#include "crc64.h"
#include <string.h>
#include <errno.h>
#include <new>
#include <glog/logging.h>

MemCache::MemCache() {
  _cachemap = NULL;
  _maxitems = 0;
  atomic64_set(&_maxsize, 0);
  atomic64_set(&_fetch, 0);
  atomic64_set(&_hit, 0);
  atomic64_set(&_put, 0);
  atomic64_set(&_random, 0);
  atomic64_set(&_malloced, 0);
  atomic64_set(&_swap, 0);
}

MemCache::~MemCache() {
  clear(_heads, _tails);
  clear(_freeheads, _freetails);
  if (_cachemap) {
    delete _cachemap;
    _cachemap = NULL;
  }
}

void MemCache::remove_lru_node(uint8_t id) {
  int64_t maxsize = atomic64_read(&_maxsize);
  if (atomic64_read(&_malloced) < maxsize) {
    return;
  }

  if (_locks[id].lock() != 0) return;
  //插入是在队列头，扫描从队尾开始，队尾插入的时间肯定比队列头的要早
  MemCacheItem *item = _tails[id];
  while (item != NULL) {
    MemCacheItem *item2 = item;
    item = item->prev;

    if (_cachemap->tryLockWrite(item2->key) != 0) continue;
    bool bRemove = false;
    if (_cachemap->remove(item2->key)) { //先删除map的key
      bRemove = true;
    }
    _cachemap->unlock(item2->key);
    if (!bRemove) continue;

    atomic64_inc(&_swap);
    unlink(item2, _heads, _tails);
    item2->is_free = true;
    link(item2, _freeheads, _freetails);
    if (item2->value) {
      delete[] item2->value;
      item2->value = NULL;
      //获取到足够的空间就结束
      if (atomic64_sub_return(item2->valuesize,
                              &_malloced) < maxsize) {
        break;
      }
    }
  }
  _locks[id].unlock();
}

//将新的item添加到it->id对应的队列中
MemCacheItem *MemCache::add(MemCacheItem *it) {
  uint32_t size = it->valuesize;
  uint8_t id = it->id;
  if (_locks[id].lock() != 0) return NULL;

  MemCacheItem *it2 = pop(id, _freeheads, _freetails);
  if (it2 != NULL) {
    it2->init(it);
  } else {
    if ((it2 = new(std::nothrow) MemCacheItem(it)) == NULL) {
      _locks[id].unlock();
      return NULL;
    }
    size += sizeof(MemCacheItem);
  }
  it2->is_free = false;
  link(it2, _heads, _tails);
  _locks[id].unlock();

  atomic64_add(size, &_malloced);
  return it2;
}

int MemCache::del(MemCacheItem *it) {
  uint8_t id = it->id;
  if (_locks[id].lock()) {
    return -1;
  }
  if (it->is_free) { //很小概率已经被lru淘汰
    _locks[id].unlock();
    return 0;
  }
  atomic64_inc(&_swap);
  unlink(it, _heads, _tails);
  if (it->value) {
    delete[] it->value;
    it->value = NULL;
    atomic64_sub(it->valuesize, &_malloced);
  }
  it->is_free = true;
  link(it, _freeheads, _freetails);
  _locks[id].unlock();
  return 0;
}

//每访问一次进行更新（插入到队列头）
int MemCache::update_r(MemCacheItem *it) {
  uint8_t id = it->id;
  if (_locks[id].lock() != 0)
    return -1;
  update(it);
  _locks[id].unlock();
  return 0;
}

int MemCache::update(MemCacheItem *it) {
  uint8_t id = it->id;
  if (!it->is_free && _heads[id] != it) {
    //要更新的节点不是head节点，而且队列最起码有2个node
    if (_tails[id] == it)
      _tails[id] = it->prev;
    if (it->next) it->next->prev = it->prev;
    it->prev->next = it->next;
    //move to head
    it->prev = NULL;
    it->next = _heads[id];
    it->next->prev = it;
    _heads[id] = it;
  }
  return 0;
}

//清空全部已经分配的内存
void MemCache::clear(MemCacheItem **heads, MemCacheItem **tails) {
  if (heads == NULL || tails == NULL) return;
  MemCacheItem *it = NULL, *it2 = NULL;
  for (uint8_t i = 0; i < CACHE_QUEUES; i++) {
    if (_locks[i].lock() == 0) {
      it = tails[i];
      while (it != NULL) {
        it2 = it;
        it = it->prev;
        unlink(it2, heads, tails);
        if (it2->value) {
          delete[] it2->value;
          it2->value = NULL;
        }
        delete it2;
      }
      heads[i] = NULL;
      tails[i] = NULL;
      _locks[i].unlock();
    }
  }
}

void MemCache::clear() {
  atomic64_set(&_fetch, 0);
  atomic64_set(&_hit, 0);
  atomic64_set(&_put, 0);
  atomic64_set(&_malloced, 0);
  atomic64_set(&_swap, 0);
  clear(_heads, _tails);
  clear(_freeheads, _freetails);
  if (_cachemap) {
    _cachemap->clear();
  }
}

//将item插入到队列中
void MemCache::link(MemCacheItem *it,
                    MemCacheItem **heads, MemCacheItem **tails) {
  //assert(it != heads[it->id]);
  //assert((heads[it->id] && tails[it->id]) ||
  //(heads[it->id] == NULL && tails[it->id] == NULL));
  uint8_t id = it->id;
  it->prev = NULL;
  it->next = heads[id];
  if (it->next)
    it->next->prev = it;
  heads[id] = it;
  if (tails[id] == NULL)
    tails[id] = it;
}

//将item从队列删除
void MemCache::unlink(MemCacheItem *it,
                      MemCacheItem **heads, MemCacheItem **tails) {
  uint8_t id = it->id;
  if (heads[id] == it)
    heads[id] = it->next;
  if (tails[id] == it)
    tails[id] = it->prev;
  //assert(it->next != it);
  //assert(it->prev != it);
  if (it->next) it->next->prev = it->prev;
  if (it->prev) it->prev->next = it->next;
}

MemCacheItem *MemCache::pop(uint8_t id,
                            MemCacheItem **heads, MemCacheItem **tails) {
  MemCacheItem *it = heads[id];
  if (it == NULL) return NULL;
  heads[id] = it->next;
  if (tails[id] == it)
    tails[id] = NULL;
  if (it->next)
    it->next->prev = NULL;
  return it;
}

//初始化操作，主要是各个对象的初始化
int MemCache::init(uint64_t maxsize, uint64_t maxitems) {
  _maxitems = maxitems;
  uint64_t nManagerSize = 0;
  if ((_cachemap = new(std::nothrow) CacheMap(maxitems)) == NULL ||
      (nManagerSize = _cachemap->init()) == 0) {
    LOG(ERROR) << "nManagerSize is " << nManagerSize;
    return -1;
  }
  nManagerSize += sizeof(CacheMap);

  uint64_t nPreLoadItems = (uint64_t) maxitems / CACHE_QUEUES;
  for (uint8_t i = 0; i < CACHE_QUEUES; i++) {
    _heads[i] = NULL;
    _tails[i] = NULL;
    _freeheads[i] = NULL;
    _freetails[i] = NULL;
    for (uint64_t j = 0; j < nPreLoadItems; j++) {
      MemCacheItem *item = new(std::nothrow) MemCacheItem(i);
      item->is_free = true;
      link(item, _freeheads, _freetails);
    }
  }

  //减去主要管理耗的内存
  size_t n = sizeof(util::Mutex) + sizeof(MemCacheItem) * nPreLoadItems;
  nManagerSize += n * CACHE_QUEUES;
  if (nManagerSize >= maxsize) {
    LOG(ERROR) << "nManagerSize is " << nManagerSize << ", maxsize is " << maxsize;
    return -2;
  }
  maxsize -= nManagerSize;
  atomic64_set(&_maxsize, maxsize);
  return 0;
}

int MemCache::put(const MCElem &mckey, const MCElem &mcvalue) {
  atomic64_inc(&_put); //插入次数++
  uint64_t hash_val = crc64(mckey.data, mckey.len);
  uint8_t id = atomic64_inc_return(&_random) % CACHE_QUEUES;
  MemCacheItem it(id, hash_val, mcvalue.data, mcvalue.len);
  /*
   * 先插入链表队列(不用_cachemap 写锁保护是防止死锁)
   * 坏处是: 极小的概率会被在执行_cachemap->insert前就被remove_lru_node淘汰掉,
   * 这样导致虽然可以被查询到，但内存已经释放，所以get结果要判断MemCacheItem.value是否为空
   */
  MemCacheItem *it2 = add(&it);
  if (it2 == NULL) return -1;

  //首先看是否已经在memcache了，如果在直接返回
  CacheItem *cache_item = NULL;
  if (_cachemap->lockWrite(hash_val) != 0)
    return -2;
  cache_item = _cachemap->find(hash_val, cache_item);
  if (cache_item) { // 更新
    _cachemap->remove(hash_val);
  }
  _cachemap->insert(it2);
  _cachemap->unlock(hash_val);

  if (cache_item) { // 删除掉老的
    del((MemCacheItem *) cache_item);
  }

  remove_lru_node(id); //如果内存已经超过规定的上限，则淘汰老的
  uint64_t nSize = _cachemap->rehash(); //如果add成功，可能需要对map做rehash操作
  if (nSize > 0) reset_maxsize(nSize);
  return 0;
}

int MemCache::get(const MCElem &mckey, MCElem &mcvalue, int nTimeout) {
  mcvalue.data = NULL;
  atomic64_inc(&_fetch); //查找次数++
  CacheItem *cache_item = NULL;
  uint64_t hash_val = crc64(mckey.data, mckey.len);
  if (_cachemap->lockRead(hash_val, nTimeout) != 0)
    return -1;
  if ((cache_item = _cachemap->find(hash_val, cache_item)) == NULL) {
    _cachemap->unlock(hash_val);
    return -2;
  }
  MemCacheItem *it = (MemCacheItem *) cache_item;
  if (it->value != NULL && it->valuesize > 0) {
    mcvalue.data = new(std::nothrow) char[it->valuesize];
    if (mcvalue.data != NULL) {
      memcpy(mcvalue.data, it->value, it->valuesize);
      mcvalue.len = it->valuesize;
    }
  }
  _cachemap->unlock(hash_val);
  atomic64_inc(&_hit);  //命中次数++
  update_r(it);  //更新该item在队列中的位置
  return 0;
}

int MemCache::del(const MCElem &mckey) {
  uint64_t hash_val = crc64(mckey.data, mckey.len);
  //首先看是否已经在memcache了，如果在直接返回
  CacheItem *cache_item = NULL;
  if (_cachemap->lockWrite(hash_val) != 0)
    return -1;
  cache_item = _cachemap->find(hash_val, cache_item);
  if (NULL == cache_item) {
    _cachemap->unlock(hash_val);
    return 0;
  }
  _cachemap->remove(hash_val);
  _cachemap->unlock(hash_val);

  if (del((MemCacheItem *) cache_item) != 0) {
    return -2;
  }

  return 0;
}

//以下是2个统计函数，主要统计查询次数，命中率和插入次数
void MemCache::getStat(stringstream &output) {
  MemCacheStat stat;
  getStat(stat);
  output << "max cache size: " << stat.nMaxSize << endl;
  output << "total fetch count: " << stat.nFetechCount << endl;
  output << "total hit count: " << stat.nHitCount << endl;
  output << "total put count: " << stat.nPutCount << endl;
  output << "cache hit ratio: " << stat.nHitRatio << "%" << endl;
  output << "total data size: " << stat.nTotalSize << endl;
  output << "total swap count: " << stat.nSwapCount << endl;
  output << "total items: " << stat.nItems << endl;
}

int MemCache::dump(const char *pFile) {
  const static float INC_FACTOR = 1.5f;
  FILE *fp = fopen(pFile, "wb");
  if (fp == NULL) return -1;
  MemCacheItem *it = NULL;
  long nSize = (long) atomic64_read(&_malloced) / CACHE_QUEUES;
  nSize = static_cast<long>(nSize * INC_FACTOR);
  long nLen = 0;
  //多分配64MB，防止越界
  char *pBuffer = new(std::nothrow) char[nSize + 64 * 1024 * 1024];
  if (pBuffer == NULL) return -2;

  for (uint8_t i = 0; i < CACHE_QUEUES; i++) {
    if (_locks[i].lock() == 0) {
      for (it = _tails[i]; it != NULL && nLen < nSize; it = it->prev) {
        int nLen2 = sizeof(uint64_t) + it->valuesize;
        long nSize2 = nLen + sizeof(int) + nLen2;
        if (nSize2 > nSize) {
          char *p = pBuffer;
          nSize = static_cast<long>(nSize * INC_FACTOR);
          if (nSize < nSize2) nSize = nSize2;
          if ((pBuffer = new(std::nothrow) char[nSize]) == NULL) {
            return -4;
          }
          memcpy(pBuffer, p, nLen);
          delete[] p;
        }
        memcpy(pBuffer + nLen, &nLen2, sizeof(int));
        nLen += sizeof(int);
        memcpy(pBuffer + nLen, &it->key, sizeof(uint64_t));
        nLen += sizeof(uint64_t);
        memcpy(pBuffer + nLen, it->value, it->valuesize);
        nLen += it->valuesize;
      }
      _locks[i].unlock();
      if (nLen > 0 && fwrite(pBuffer, nLen, 1, fp) != 1) {
        return -5;
      }
      nLen = 0;
    } else {
      return -3;
    }
  }
  return fclose(fp);
}

void MemCache::print() {
  stringstream output;
  getStat(output);
  printf("%s", output.str().c_str());

  MemCacheItem *it = NULL;
  for (uint8_t i = 0; i < CACHE_QUEUES; i++) {
    if (_locks[i].lock() == 0) {
      printf("queue %d ------------------------------ \n", i);
      for (it = _tails[i]; it != NULL; it = it->prev) {
        it->print();
      }
      printf("\n");
      _locks[i].unlock();
    }
  }
}
