#ifndef _UTIL_THREAD_LOCK_H_
#define _UTIL_THREAD_LOCK_H_

#include <pthread.h>
#include <stdint.h>
#include <sys/time.h>
#include <time.h>
#include "global_def.h"

#ifndef __USE_PTHREAD_RWLOCK
#define __USE_PTHREAD_RWLOCK
#endif

namespace util {

class Mutex {
 public:
  Mutex();
  ~Mutex();
  //截止超时的绝对时间，NULL代表永远不超时
  int32_t lock(timespec *pEndTime = NULL);
  int32_t trylock();
  int32_t unlock();
 private:
  pthread_mutex_t _inst;
};

class MutexGuard {
 public:
  explicit MutexGuard(Mutex &lock, timespec *pEndTime = NULL) : _lock(lock) {
    _lock.lock(pEndTime);
  }
  ~MutexGuard() {
    _lock.unlock();
  }
 private:
  Mutex &_lock;
};

class RWLock {
 public:
  RWLock();
  ~RWLock();
  //截止超时的绝对时间，0代表永远不超时
  int32_t rdlock(timespec *pEndTime = NULL);
  int32_t wrlock(timespec *pEndTime = NULL);
  int32_t tryrdlock();
  int32_t trywrlock();
  int32_t unlock();
 private:
#ifdef __USE_PTHREAD_RWLOCK
  pthread_rwlock_t _inst;
#else
  pthread_mutex_t _inst;
#endif
};

class RdLockGuard {
 public:
  explicit RdLockGuard(RWLock &lock, timespec *pEndTime = NULL) : _lock(lock) {
    _lock.rdlock(pEndTime);
  }
  ~RdLockGuard() {
    _lock.unlock();
  }
 private:
  RWLock &_lock;
};

class WrLockGuard {
 public:
  explicit WrLockGuard(RWLock &lock, timespec *pEndTime = NULL) : _lock(lock) {
    _lock.wrlock(pEndTime);
  }
  ~WrLockGuard() {
    _lock.unlock();
  }
 private:
  RWLock &_lock;
};

class Condition {
 public:
  Condition();
  ~Condition();
  int32_t lock();
  int32_t unlock();
  int32_t wait();
  int32_t timedwait(uint32_t ms);
  int32_t signal();
  int32_t broadcast();
 private:
  pthread_mutex_t _lock;
  pthread_cond_t _cond;
};

class ConditionGuard {
 public:
  explicit ConditionGuard(Condition cond) : _cond(cond) {
    _cond.lock();
  }
  ~ConditionGuard() {
    _cond.unlock();
  }
 private:
  Condition &_cond;
};

class Conditions {
 public:
  Conditions(uint32_t n);
  ~Conditions();
  int32_t lock();
  int32_t unlock();
  int32_t wait(uint32_t idx);
  int32_t timedwait(uint32_t idx, uint32_t ms);
  int32_t signal(uint32_t idx);
  int32_t broadcast(uint32_t idx);
 private:
  uint32_t _unCount;
  pthread_cond_t *_conds;
  pthread_mutex_t _lock;
};

class BoolCondition {
 public:
  BoolCondition();
  ~BoolCondition();
  int32_t setBusy();
  int32_t waitIdle();
  bool isBusy();
  int32_t clearBusy();
  int32_t clearBusyBroadcast();
 private:
  bool _bBusy;
  Condition _cond;
};

class SpinLock {
 public:
  SpinLock() {
    pthread_spin_init(&_lock, 0);
  }
  ~SpinLock() {
    pthread_spin_destroy(&_lock);
  }

  int lock() {
    return pthread_spin_lock(&_lock);
  }

  int unlock() {
    return pthread_spin_unlock(&_lock);
  }

  int trylock() {
    return pthread_spin_trylock(&_lock);
  }

 private:
  pthread_spinlock_t _lock;
};

}

#endif //_THREAD_LOCK_H_
