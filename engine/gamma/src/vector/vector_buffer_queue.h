#ifndef VECTOR_BUFFER_QUEUER_H_
#define VECTOR_BUFFER_QUEUER_H_

#include <cstdint>
#include <pthread.h>

class VectorBufferQueue {
public:
  /**
   * @param capacity the max memory buffer size, the smallest unit is million(M)
   * bytes, for example: capacity=100, it means 100M bytes
   * @param dimension the dimension for each vector, for example: dimemison=1024
   * @return
   */
  VectorBufferQueue(int max_size, int resident_size, int dimension);
  ~VectorBufferQueue();

  /**
   * malloc memory, init variables
   * @return 0 success; 1 parameter error; 2 malloc memory error;
   */
  int Init(int init_index);

  /**
   * add one vector to queue
   * @param v the float array of vector
   * @param dim the dimension of vector, it must be equal to dimension of
   * constructor
   * @param timeout the timeout of waiting enough space to store this vector, -1
   * means waiting forever, the smallest unit is millisecond(ms), for example:
   * timeout=100, it means to waiting 100ms
   * @return 0 success; 1 parameter error; 3 timeout
   */
  int Add(const float *v, int dim, int timeout);

  /**
   * add multiple vector to queue
   * @param v the float array of all multiple vector
   * @param dim the dimension of each vector, it must be equal to dimension of
   * constructor
   * @num the number of vector
   * @param timeout the timeout of waiting enough space to store this vector, -1
   * means waiting forever, the smallest unit is millisecond(ms), for example:
   * timeout=100, it means to waiting 100ms
   * @return 0 success; 1 parameter error; 3 timeout
   */
  int Add(const float *v, int dim, int num, int timeout); // batch add

  /**
   * poll one vector from queue
   * @param v the float array to store vector
   * @param dim the dimension of vector, it is equal to dimension of constructor
   * @param timeout the timeout of waiting enough vector to poll from the queue,
   * -1 means waiting forever, the smallest unit is millisecond(ms), for
   * example: timeout=100, it means to waiting 100ms
   * @return 0 success; 1 parameter error; 3 timeout
   */
  int Poll(float *v, int *dim, int timeout);

  /**
   * poll multiple vector from queue
   * @param v the float array to store multiple vector
   * @param dim the dimension of each vector, it is equal to dimension of
   * constructor
   * @num the number of vector to poll
   * @param timeout the timeout of waiting enough vector to poll from the queue,
   * -1 means waiting forever, the smallest unit is millisecond(ms), for
   * example: timeout=100, it means to waiting 100ms
   * @return 0 success; 1 parameter error; 3 timeout
   */
  int Poll(float *v, int *dim, int num, int timeout); // batch poll

  // TODO: id should be int64 ?
  int GetVector(int id, float *v, int dim);
  int Peek(float *v, int dim, int num) const;
  int PeekSize() const;
  int Size() const;
  void MovePollIndex(int inc);
  void MovePeekIndex(int inc);

  std::uint64_t GetPollIndex() const { return poll_index_; };
  std::uint64_t GetAddIndex() const { return add_index_; };
  std::uint64_t GetPeekIndex() const { return peek_index_; };

  long GetTotalMemBytes() {return total_mem_bytes_;};

private:
  bool WaitFor(int timeout, int type, int num);

private:
  float *buffer_;
  int max_size_;
  int resident_size_;
  int dimension_;
  std::uint64_t poll_index_;
  std::uint64_t add_index_;
  std::uint64_t peek_index_;
  int feature_byte_size_;
  pthread_rwlock_t shared_mutex_;
  long total_mem_bytes_;
};

#endif
