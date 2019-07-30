#ifndef __CACHEMAP_H__
#define __CACHEMAP_H__

#include <assert.h>
#include <stdio.h>
#include "atomic.h"
#include <stdint.h>
#include "thread_lock.h"

struct CacheItem {
  CacheItem() {
    reset();
  }
  virtual ~CacheItem() {
    reset();
  }
  void reset() {
    key = 0;
    hash_next = NULL;
  }
  uint64_t key;        //��item����Ĳ�ѯkey
  CacheItem *hash_next; //��map�ж�Ӧ��nextָ��
};

typedef CacheItem HashEntry;
typedef HashEntry *HashEntryPtr;

class CacheMap {
 private:
  uint64_t _nHashSize;    //Bucket�����С
  uint64_t _nLockSize;    //lock�����С
  uint64_t _nUsedSize;    //cachemap�Է�����ڴ�size
  atomic64_t _nHashUsed;    //�Ѿ�ʹ�õ�Ԫ�ظ���
  uint64_t _nRehash;        //��Ҫ����rehash����ֵ
  HashEntryPtr *_ppEntry;    //Bucket����,���һ������entryָ��
  util::RWLock *_pBucketLock;   //hashͰ��д��
  util::RWLock *_pNodeLock;     //hashͰ�ڵ��д��

 public:
  /* ���캯��, ֱ��ָ����Ҫ���Ŀռ������Ԫ�� */
  CacheMap(uint64_t nHashSize);
  /* �������� */
  virtual ~CacheMap(void);
  /* ���س�ʼ�����ĵ��ڴ�size */
  uint64_t init(void);

 public:
  /* �������в���key��value */
  bool insert(HashEntryPtr item);
  /* ����key���ĳһ��Ԫ�� */
  bool remove(const uint64_t &nKey);
  /* ��������е�����Ԫ�� */
  void clear(void);
  /* ����key����valueֵ,��key�������в�����ʱ,����noneĬ��ֵ */
  HashEntryPtr find(const uint64_t &nKey, const HashEntryPtr _none);
  /* ����������Ԫ�صĸ��� */
  uint64_t size() const;
  /* ������Ԫ�صĴ�С������������3/5,��Ҫ���¹����hashmap, �������ӵ��ڴ�size */
  uint64_t rehash();
  /* printf hash size*/
  void print();
  uint64_t getBucket(uint64_t nNum);
  uint64_t capacity() {
    return _nUsedSize;
  }

  //nTimeout: ����ʱʱ�䣬0����������ʱ����ͬ��
  int lockRead(const uint64_t &nKey, int nTimeout = 0, bool lock_bucket = true);
  int lockWrite(const uint64_t &nKey, int nTimeout = 0, bool lock_bucket = true);
  int tryLockRead(const uint64_t &nKey, bool lock_bucket = true);
  int tryLockWrite(const uint64_t &nKey, bool lock_bucket = true);
  int unlock(const uint64_t &nKey, bool lock_bucket = true);

  int lockReadBucket(struct timespec *pEndTime = NULL);
  int lockWriteBucket(struct timespec *pEndTime = NULL);
  int unlockBucket();

  HashEntryPtr *getAllEntitys(uint64_t &nSize) {
    nSize = _nHashSize;
    return _ppEntry;
  }

 private:
  bool isPrime(uint64_t n);
};

#endif

