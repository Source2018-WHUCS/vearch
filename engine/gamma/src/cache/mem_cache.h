/*
 *  Memory Cache for C++, use LRU algorithm.
 *  by using the double linked list and thread-safe hashmap
 */

#ifndef __MEMORY_CACHE_H_
#define __MEMORY_CACHE_H_

#include <sstream>
#include "cache_map.h"
#include "thread_lock.h"

using namespace std;

/**
 * 总共分n个队列，轮询保存放到memcache的item。
 * 多队列是为了解决1个队列加锁时间过长的问题，可能会损失hit_ratio
 */
const static uint8_t CACHE_QUEUES = 32;

struct MemCacheStat {
  MemCacheStat() : nMaxSize(0), nTotalSize(0), nFetechCount(0),
                   nHitCount(0), nPutCount(0), nHitRatio(0),
                   nSwapCount(0), nItems(0) {}
  uint32_t nMaxSize;        //memory cache的上限, 单位MB
  uint32_t nTotalSize;    //内存cache当前占用的空间大小, 单位MB
  uint64_t nFetechCount;  //取cache的count
  uint64_t nHitCount;     //命中的数量
  uint64_t nPutCount;     //存放的次数
  double nHitRatio;        //命中率
  uint64_t nSwapCount;    //替换出去的次数
  uint64_t nItems;        //当前内存存的个数
};

struct MCElem {
  MCElem() : data(NULL), len(0) {}
  MCElem(char *data, uint32_t len) {
    this->data = data;
    this->len = len;
  }
  char *data;
  uint32_t len;
};

struct MemCacheItem : public CacheItem {
  MemCacheItem(uint8_t id = 0, uint64_t key = 0,
               char *value = NULL, uint32_t valuesize = 0,
               bool is_free = false) {
    this->id = id;
    this->key = key;
    this->is_free = is_free;
    this->value = value;
    this->valuesize = valuesize;
    next = NULL;
    prev = NULL;
  }

  MemCacheItem(const MemCacheItem *item) {
    init(item);
  }

  void init(const MemCacheItem *item) {
    key = item->key;
    next = item->next;
    prev = item->prev;
    is_free = item->is_free;
    id = item->id;
    valuesize = item->valuesize;
    value = item->value;
  }

  void print() {
    printf("id = %d, value=%s\n", id, value);
  }

  MemCacheItem *next;  //在队列中对应的next指针
  MemCacheItem *prev;  //在队列中对应的prev指针
  uint8_t id;           //item的队列id，用于标识属于哪个队列
  bool is_free;         //是否处在free队列中
  uint32_t valuesize;
  char *value;
};

/* 
 * memcache 设计以加快速度为最主要目的，扩展性次要
 * 原则是尽量缩短每个锁的空间 
 */
class MemCache {
 public:
  MemCache();
  virtual ~MemCache();

  /*
   * 初始化指定最大内存size和items上限，
   * 这里的maxitems只是给map桶赋一个合适的初值，并不实际作用
   */
  int init(uint64_t maxsize, uint64_t maxitems);

  /*
   * 将查询结果放到memcache中, 0代表返回成功，其它失败
   * 直接复用mcvalue.data的内存
   */
  int put(const MCElem &mckey, const MCElem &mcvalue);

  /*
   * 从memcache获取mckey对应的结果, 并放在mcvalue中
   * 返回0成功，其它失败. 如果成功，由调用者负责释放内存（delete[] mcvalue.data)
   * timeout查询超时时间，单位ms, 0代表永远不超时
   */
  int get(const MCElem &mckey, MCElem &mcvalue, int timeout = 0);

  //从memcache中删除掉mckey
  int del(const MCElem &mckey);

  //统计函数
  void getStat(stringstream &output);
  inline void getStat(MemCacheStat &stat) {
    stat.nMaxSize = atomic64_read(&_maxsize) >> 20;
    stat.nTotalSize = atomic64_read(&_malloced) >> 20;
    stat.nFetechCount = atomic64_read(&_fetch);
    stat.nHitCount = atomic64_read(&_hit);
    stat.nPutCount = atomic64_read(&_put);
    int64_t fetch = stat.nFetechCount > 0 ? stat.nFetechCount : 1;
    stat.nHitRatio = 100 * ((double) stat.nHitCount / fetch);
    stat.nSwapCount = atomic64_read(&_swap);
    stat.nItems = _cachemap->size();
  }

  //dump cache内容
  int dump(const char *pFile);

  //打印函数
  void print();
  //从未验证过，谨慎使用
  void clear();

 private:
  //调用link将item插入到队列中, 返回最终使用的内存地址
  MemCacheItem *add(MemCacheItem *it);

  //删除指定的item
  int del(MemCacheItem *it);

  //访问一次后更新item
  int update_r(MemCacheItem *it);  //线程安全
  int update(MemCacheItem *it);    //线程不安全

  //重设最大的size限制，nSize是变化的量，一般是会减少最大size
  void reset_maxsize(uint64_t nSize) { atomic64_sub(nSize, &_maxsize); }

  //将item插入到队列中
  void link(MemCacheItem *it, MemCacheItem **heads, MemCacheItem **tails);
  //将item从队列中删除
  void unlink(MemCacheItem *it, MemCacheItem **heads, MemCacheItem **tails);

  //当内存不足时，移除最近未被访问的节点
  void remove_lru_node(uint8_t id);

  //释放memcache保存的所有item
  void clear(MemCacheItem **heads, MemCacheItem **tails);

  //弹出id所在队列的头元素
  MemCacheItem *pop(uint8_t id, MemCacheItem **heads, MemCacheItem **tails);

 private:
  uint64_t _maxitems;   //最大items
  atomic64_t _maxsize;  //内存上限(byte)
  atomic64_t _malloced; //已经使用的内存字节数
  atomic64_t _swap;     //用于统计cacheitem被交换出去的次数
  atomic64_t _fetch;    //取cache的count
  atomic64_t _hit;      //命中的数量
  atomic64_t _put;      //存放的次数
  atomic64_t _random;   //用于队列轮询的id，每次加1

 private:
  CacheMap *_cachemap; //和memcache队列配合使用的cachemap
  MemCacheItem *_heads[CACHE_QUEUES]; //CACHE_QUEUES个队列头指针
  MemCacheItem *_tails[CACHE_QUEUES]; //CACHE_QUEUES个队列为指针
  MemCacheItem *_freeheads[CACHE_QUEUES]; //CACHE_QUEUES个回收队列头指针
  MemCacheItem *_freetails[CACHE_QUEUES]; //CACHE_QUEUES个回收队列为指针
  util::Mutex _locks[CACHE_QUEUES]; //用于保护每个队列的锁
};

#endif

