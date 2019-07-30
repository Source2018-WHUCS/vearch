#include "cache_map.h"
#include <math.h>
#include <new>
#include <glog/logging.h>
#include "global_def.h"

#define HASH_LIMIT(n) ((uint64_t) (n * 6)/4)

bool CacheMap::isPrime(uint64_t n) {
  if (n <= 1) return false;
  uint64_t nSqrt = static_cast<uint64_t>(sqrt(n));
  for (uint64_t i = 2; i <= nSqrt; ++i) {
    if (n % i == 0) {
      return false;
    }
  }
  return true;
}

uint64_t CacheMap::getBucket(uint64_t nNum) {
  const static float LOAD_FACTOR = 0.75f;
  uint64_t nNum2 = static_cast<uint64_t>(nNum * LOAD_FACTOR);
  const static uint64_t DEFAULT_BUCKET = 1024;
  if (nNum2 < DEFAULT_BUCKET) return DEFAULT_BUCKET;
  for (int i = 0; i < 100000; i++) {
    uint64_t m = nNum2 + i;
    if (isPrime(m)) {
      return m;
    }
  }
  return nNum2;
}

/* ���캯��, ֱ��ָ����Ҫ���Ŀռ������Ԫ�� */
CacheMap::CacheMap(uint64_t nHashSize) {
  _nRehash = nHashSize;
  _nHashSize = getBucket(nHashSize);
  _nLockSize = (uint64_t) _nHashSize / 10000;
  if (_nLockSize == 0) _nLockSize = 128;
  atomic64_set(&_nHashUsed, 0);
  _ppEntry = NULL;
  _pBucketLock = NULL;
  _pNodeLock = NULL;
  _nUsedSize = 0;
}

/* �������� */
CacheMap::~CacheMap(void) {
  if (_ppEntry) {
    delete[] _ppEntry;
    _ppEntry = NULL;
  }
  if (_pBucketLock) {
    delete _pBucketLock;
    _pBucketLock = NULL;
  }
  if (_pNodeLock) {
    delete[] _pNodeLock;
    _pNodeLock = NULL;
  }
  _nUsedSize = 0;
}

uint64_t CacheMap::init(void) {
  uint64_t nPrevUsedSize = _nUsedSize;
  _ppEntry = new(std::nothrow) HashEntryPtr[_nHashSize];
  if (_ppEntry == NULL) {
    LOG(ERROR) << "CacheMap: new failed!";
    return 0;
  }
  uint64_t nSize = _nHashSize * sizeof(HashEntryPtr);
  memset(_ppEntry, 0, nSize);

  _nUsedSize += nSize;
  _pBucketLock = new(std::nothrow) util::RWLock();
  if (_pBucketLock == NULL) {
    LOG(ERROR) << "CacheMap: new failed!";
    return 0;
  }
  _nUsedSize += sizeof(util::RWLock);
  _pNodeLock = new(std::nothrow) util::RWLock[_nLockSize];
  if (_pNodeLock == NULL) {
    LOG(ERROR) << "CacheMap: new failed!";
    return 0;
  }
  _nUsedSize += sizeof(util::RWLock) * (_nLockSize);
  return (_nUsedSize - nPrevUsedSize);
}

int CacheMap::lockRead(const uint64_t &nKey, int nTimeout, bool lock_bucket) {
  struct timespec nEndTime;
  struct timespec *pEndTime = NULL;
  if (nTimeout > 0) {
    struct timeval nNowTime;
    gettimeofday(&nNowTime, NULL);
    nEndTime.tv_sec = nNowTime.tv_sec + nTimeout / 1000;
    nEndTime.tv_nsec = (nNowTime.tv_usec + (nTimeout % 1000) * 1000) * 1000;
    pEndTime = &nEndTime;
  }

  int ret = 0;
  if (lock_bucket) {
    if ((ret = lockReadBucket(pEndTime)) != 0) {
      return ret;
    }
  }
  uint64_t nPos = nKey % _nHashSize;
  ret = _pNodeLock[nPos % _nLockSize].rdlock(pEndTime);
  if (ret != 0 && lock_bucket) {
    unlockBucket();
  }
  return ret;
}

int CacheMap::lockWrite(const uint64_t &nKey, int nTimeout, bool lock_bucket) {
  struct timespec nEndTime;
  struct timespec *pEndTime = NULL;
  if (nTimeout > 0) {
    struct timeval nNowTime;
    gettimeofday(&nNowTime, NULL);
    nEndTime.tv_sec = nNowTime.tv_sec + nTimeout / 1000;
    nEndTime.tv_nsec = (nNowTime.tv_usec + (nTimeout % 1000) * 1000) * 1000;
    pEndTime = &nEndTime;
  }

  int ret = 0;
  if (lock_bucket) {
    if ((ret = lockReadBucket(pEndTime)) != 0) {
      return ret;
    }
  }
  uint64_t nPos = nKey % _nHashSize;
  ret = _pNodeLock[nPos % _nLockSize].wrlock(pEndTime);
  if (ret != 0 && lock_bucket) {
    unlockBucket();
  }
  return ret;
}

int CacheMap::tryLockRead(const uint64_t &nKey, bool lock_bucket) {
  int ret = 0;
  if (lock_bucket) {
    if ((ret = lockReadBucket()) != 0) {
      return ret;
    }
  }
  uint64_t nPos = nKey % _nHashSize;
  ret = _pNodeLock[nPos % _nLockSize].tryrdlock();
  if (ret != 0 && lock_bucket) {
    unlockBucket();
  }
  return ret;
}

int CacheMap::tryLockWrite(const uint64_t &nKey, bool lock_bucket) {
  int ret = 0;
  if (lock_bucket) {
    if ((ret = lockReadBucket()) != 0) {
      return ret;
    }
  }
  uint64_t nPos = nKey % _nHashSize;
  ret = _pNodeLock[nPos % _nLockSize].trywrlock();
  if (ret != 0 && lock_bucket) {
    unlockBucket();
  }
  return ret;
}

int CacheMap::unlock(const uint64_t &nKey, bool lock_bucket) {
  uint64_t nPos = nKey % _nHashSize;
  int ret = _pNodeLock[nPos % _nLockSize].unlock();
  if (lock_bucket) unlockBucket();
  return ret;
}

int CacheMap::lockReadBucket(struct timespec *pEndTime) {
  return _pBucketLock->rdlock(pEndTime);
}

int CacheMap::lockWriteBucket(struct timespec *pEndTime) {
  return _pBucketLock->wrlock(pEndTime);
}

int CacheMap::unlockBucket() {
  return _pBucketLock->unlock();
}

void CacheMap::print() {
  for (uint64_t i = 0; i < _nHashSize; i++) {
    uint64_t nSize = 0;
    HashEntry *pEntry = _ppEntry[i];
    while (pEntry != NULL) {
      nSize++;
      pEntry = pEntry->hash_next;
    }
    printf("hash i=%lu, nSize=%lu.\n", i, nSize);
  }
  printf("Hash map size=%lu.\n", this->size());
}

/* �������в���key��value */
// �������Ƿ��Ѿ����ڣ�����Χ�Ѿ������ж�
bool CacheMap::insert(HashEntry *item) {
  uint64_t nPos = item->key % _nHashSize;
  item->hash_next = _ppEntry[nPos];
  _ppEntry[nPos] = item;
  atomic64_inc(&_nHashUsed);
  return true;
}

/* ����key���ĳһ��Ԫ�� */
bool CacheMap::remove(const uint64_t &nKey) {
  uint64_t nPos = nKey % _nHashSize;
  HashEntry *pPrevEntry = NULL, *pCurEntry = _ppEntry[nPos];

  while (pCurEntry != NULL && nKey != pCurEntry->key) {
    pPrevEntry = pCurEntry;
    pCurEntry = pCurEntry->hash_next;
  }

  bool ret = false;
  if (pCurEntry != NULL) {
    ret = true;
    if (pPrevEntry == NULL) {
      _ppEntry[nPos] = pCurEntry->hash_next;
    } else {
      pPrevEntry->hash_next = pCurEntry->hash_next;
    }
    atomic64_dec(&_nHashUsed);
  }
  return ret;
}

/* ��������е�����Ԫ��,��������С�����ı� */
void CacheMap::clear(void) {
  if (lockWriteBucket() != 0) return;
  if (atomic64_read(&_nHashUsed) > 0) {
    memset(_ppEntry, 0, _nHashSize * sizeof(HashEntry *));
  }
  atomic64_set(&_nHashUsed, 0);
  unlockBucket();
}

/* ����key����valueֵ,��key�������в�����ʱ,����noneĬ��ֵ */
HashEntryPtr CacheMap::find(const uint64_t &nKey, const HashEntryPtr _none) {
  HashEntryPtr pEntry = _ppEntry[nKey % _nHashSize];
  while (pEntry != NULL && nKey != pEntry->key) {
    pEntry = pEntry->hash_next;
  }

  if (pEntry != NULL) {
    return pEntry;
  } else {
    return _none;
  }
}

/* ����������Ԫ�صĸ��� */
uint64_t CacheMap::size() const {
  return atomic64_read(&_nHashUsed);
}

//������Ԫ�صĴ�С��������, ��Ҫ���¹����hashmap
//�������ӵ��ڴ�size
uint64_t CacheMap::rehash() {
  /* �ж������Ƿ񳬹�����������rehash */
  if ((uint64_t) atomic64_read(&_nHashUsed) < _nRehash) {
    return 0;
  }
  if (lockWriteBucket() != 0) return 0;
  uint64_t nNewRehash = HASH_LIMIT(_nRehash);
  uint64_t nNewHashSize = getBucket(nNewRehash);

  HashEntryPtr *ppNewEntries = new(std::nothrow)
      HashEntryPtr[nNewHashSize];
  if (!ppNewEntries) {
    unlockBucket();
    return 0;
  }
  memset(ppNewEntries, 0, sizeof(HashEntryPtr) * nNewHashSize);

  uint64_t nKey;
  for (uint64_t i = 0; i < _nHashSize; i++) {
    HashEntry *pEntry = _ppEntry[i], *pEntry2 = NULL;
    while (pEntry) {
      nKey = pEntry->key % nNewHashSize;
      pEntry2 = pEntry;
      pEntry = pEntry->hash_next;
      pEntry2->hash_next = ppNewEntries[nKey];
      ppNewEntries[nKey] = pEntry2;
    }
  }
  HashEntryPtr *ppOldEntries = _ppEntry;
  _ppEntry = ppNewEntries;
  uint64_t nSize = _nUsedSize;
  _nUsedSize += (nNewHashSize - _nHashSize) * sizeof(HashEntryPtr);
  nSize = _nUsedSize - nSize;
  _nHashSize = nNewHashSize;
  _nRehash = nNewRehash;
  unlockBucket();
  delete[] ppOldEntries;
  return nSize;
}
