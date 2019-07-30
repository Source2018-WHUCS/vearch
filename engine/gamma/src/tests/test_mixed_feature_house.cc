#include "vector_file_mapper.h"
#include "memory_disk_raw_vector.h"
#include <string>
#include <sys/types.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <cassert>
#include <cmath>
#include <unistd.h>
#include <iostream>
#include "utils.h"
#include <string.h>
#include <cstdlib>
#include <ctime>
#include <thread>

using namespace tig_gamma;
using namespace std;

float *buildVector(int dim, float offset) {
  float *v = new float[dim];
  for (int i = 0; i < dim; i++) {
    v[i] = i + offset;
  }
  return v;
}

bool floatArrayEquals(const float *a, int m, const float *b, int n) {
  if (m != n) return false;
  for (int i = 0; i < m; i++) {
    if (std::fabs(a[i] - b[i]) > 0.0001f) {
      return false;
    }
  }
  return true;
}

void TestFileMapper() {
  string file_path = "test_file_mamper.fet";
  int offset = 9;
  int max_size = 1000;
  int dimension = 512;

  int fd = open(file_path.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 00777);
  assert(-1 != fd);
  assert(offset == write(fd, "headlines", offset));
  close(fd);

  VectorFileMapper *mapper =
      new VectorFileMapper(file_path, offset, max_size, dimension);
  assert(0 == mapper->Map());
  assert(0 == mapper->GetMappedNum());

  int feature_size = sizeof(float) * dimension;
  fd = open(file_path.c_str(), O_WRONLY | O_APPEND);
  for (int i = 0; i < max_size; i++) {
    float *feature = buildVector(dimension, i);
    assert(feature_size == write(fd, (void *)feature, feature_size));
    delete[] feature;
    cerr << "i=" << i << endl;
  }

  const float *features = mapper->GetVectors();
  cerr << "features[0]=" << *(features - 1) << endl;
  assert(nullptr != features);
  for (int i = 0; i < max_size; i++) {
    const float *map_feature = features + (dimension * i);
    float *expect = buildVector(dimension, i);
    cerr << "float array equal, i=" << i << ", feature=[" << map_feature[0]
         << ", " << map_feature[1] << ", " << map_feature[2] << "]"
         << ", expect=[" << expect[0] << ", " << expect[1] << ", " << expect[2]
         << "]" << endl;
    assert(floatArrayEquals(expect, dimension, map_feature, dimension));
    delete[] expect;
  }

  delete mapper;
}

void TestFileMapperLoad() {
  string file_path = "test.fet";
  int offset = 9;
  int max_size = 1000000;
  int dimension = 512;
  VectorFileMapper *mapper =
      new VectorFileMapper(file_path, offset, max_size, dimension);
  assert(0 == mapper->Map());
  int map_num = mapper->GetMappedNum();
  const float *features = mapper->GetVectors();
  cerr << "map_num=" << map_num << endl;
  const float *feature = new float[dimension];
  int fea_len = sizeof(float) * dimension;
  double begin = utils::getmillisecs();
  for (int i = 0; i < map_num; i++) {
    memcpy((void *)feature, features + i * dimension, fea_len);
    cerr << "id=" << i << ", feature[0]=" << feature[0] << endl;
  }
  cerr << "memory copy finished, cost=" << utils::getmillisecs() - begin
       << endl;
}

void TestFileMapperRandRead() {
  string file_path = "test.fet";
  int offset = 9;
  int max_size = 1000000;
  int dimension = 512;
  VectorFileMapper *mapper =
      new VectorFileMapper(file_path, offset, max_size, dimension);
  assert(0 == mapper->Map());
  std::this_thread::sleep_for(std::chrono::milliseconds(20000));
  int map_num = mapper->GetMappedNum();
  cerr << "mmap finished, map num=" << map_num << endl;
  const float *features = mapper->GetVectors();
  int times = 1000000;
  std::srand(std::time(nullptr));
  const float *feature = new float[dimension];
  int fea_len = sizeof(float) * dimension;
  double begin = utils::getmillisecs();
  for (int i = 0; i < times; i++) {
    int id = std::rand();
    id = id % map_num;
    memcpy((void *)feature, features + id * dimension, fea_len);
    cerr << "i=" << i << ", id=" << id << ", feature[0]=" << feature[0] << endl;
  }
  cerr << "file mapper random read finished, cost="
       << utils::getmillisecs() - begin << "ms, times=" << times << endl;
}

void TestMemoryDisckRawFeature() {
  string file_path = "test_memory_disk.fet";
  int offset = 9;
  int max_size = 100000;
  int dimension = 512;
  int resident_buffer_size = 1000;
  int max_buffer_size = resident_buffer_size * 2;

  int fd = open(file_path.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 00777);
  assert(-1 != fd);
  assert(offset == write(fd, "headlines", offset));
  close(fd);

  MemoryDiskRawVector *raw_feature = new MemoryDiskRawVector(
      "mixed_test", max_size, dimension, resident_buffer_size, max_buffer_size);
  raw_feature->SetFilePath(file_path);
  raw_feature->SetOffset(offset);
  // raw_feature->SetFlushBatchSize(500);
  assert(0 == raw_feature->Init());

  // int AddWithIds(int n, const float *x, const long *xids, int timeout)
  int xids[1];
  int doc_num = 50000;
  for (int i = 0; i < doc_num; i++) {
    xids[0] = i;
    float *feature = buildVector(dimension, i);
    cerr << "add i=" << i << endl;
    assert(0 == raw_feature->AddWithIds(1, feature, xids, -1));
  }

  int added_doc_num = raw_feature->GetVectorNum();
  assert(doc_num == added_doc_num);
  const float *all_features = raw_feature->GetVectors(added_doc_num);
  for (int i = 0; i < doc_num; i++) {
    float *expect = buildVector(dimension, i);
    const float *map_feature = all_features + i * dimension;
    cerr << "float array equal, i=" << i << ", feature=[" << map_feature[0]
         << ", " << map_feature[1] << ", " << map_feature[2] << "]"
         << ", expect=[" << expect[0] << ", " << expect[1] << ", " << expect[2]
         << "]" << endl;
    assert(floatArrayEquals(expect, dimension, map_feature, dimension));
    delete[] expect;
  }

  raw_feature->Close();
}

int CreateFeatureFile(string file_path, string head, int max_size,
                      int dimension) {
  int fd = open(file_path.c_str(), O_WRONLY | O_CREAT | O_TRUNC, 00777);
  assert(-1 != fd);
  assert(head.length() == write(fd, head.c_str(), head.length()));
  close(fd);

  int feature_size = sizeof(float) * dimension;
  fd = open(file_path.c_str(), O_WRONLY | O_APPEND);
  for (int i = 0; i < max_size; i++) {
    float *feature = buildVector(dimension, i + 0.1f);
    assert(feature_size == write(fd, (void *)feature, feature_size));
    delete[] feature;
    if (i % 10000 == 0) {
      cerr << "create feature file i=" << i << endl;
    }
  }
  close(fd);
  cerr << "create feature file success, file path=" << file_path
       << ", max size=" << max_size << ", head=" << head
       << ", dimension=" << dimension << endl;
  return 0;
}

void TestMemoryDiskRawFeatureRandomGet(int m_size, int r_times) {
  string file_path = "test_memory_disk.fet";
  string head = "headlines";
  int offset = head.length();
  int max_size = 10000;
  int dimension = 512;
  int read_times = 10000;
  int fea_len = sizeof(float) * dimension;
  if (m_size != 0) {
    max_size = m_size;
  }
  if (r_times != 0) {
    read_times = r_times;
  }
  if (access(file_path.c_str(), F_OK) != 0) {
    cerr << "file_path=" << file_path << " is not existed, create it" << endl;
    assert(0 == CreateFeatureFile(file_path, head, max_size, dimension));
  } else {
    cerr << "file_path=" << file_path << " is already existed, reuse it!"
         << endl;
    long file_size = utils::get_file_size(file_path.c_str());
    if ((file_size - offset) % fea_len != 0) {
      cerr << "invalid file size=" << file_size << endl;
      assert(0 == 1);
    }
    max_size = (file_size - offset) / fea_len;
  }
  cerr << "file_path=" << file_path << ", head=" << head
       << ", max_size=" << max_size << ", dimension=" << dimension
       << ", read times=" << read_times << endl;

  int resident_buffer_size = 1000;
  int max_buffer_size = resident_buffer_size * 2;
  MemoryDiskRawVector *raw_feature = new MemoryDiskRawVector(
      "mixed_test", max_size, dimension, resident_buffer_size, max_buffer_size);
  raw_feature->SetFilePath(file_path);
  raw_feature->SetOffset(offset);
  // raw_feature->SetFlushBatchSize(500);
  assert(0 == raw_feature->Init());

  std::srand(std::time(nullptr));
  // float *feature = new float[dimension];
  double begin = utils::getmillisecs();
  for (int i = 0; i < read_times; i++) {
    int id = std::rand();
    id = id % max_size;
    const float *feature = raw_feature->GetVector(id);
    cerr << "i=" << i << ", id=" << id << ", feature[0]=" << feature[0] << endl;
    raw_feature->DestroyVector(feature);
  }
  cerr << "rand read finished, times=" << read_times
       << ", cost=" << utils::getmillisecs() - begin << "ms" << endl;
}

void TestFeatureBufferQueue() {
  int dimension = 512;
  int resident_buffer_size = 1000;
  int max_buffer_size = 2000;

  VectorBufferQueue *queue =
      new VectorBufferQueue(max_buffer_size, resident_buffer_size, dimension);
  assert(0 == queue->Init(0));
  assert(0 == queue->PeekSize());
  assert(0 == queue->Size());

  int doc_num = 2000;
  for (int i = 0; i < doc_num; i++) {
    float *feature = buildVector(dimension, i);
    // cerr << "add i=" << i << endl;
    assert(0 == queue->Add(feature, dimension, -1));
    delete[] feature;
  }
  assert(doc_num == queue->PeekSize());
  assert(doc_num == queue->Size());

  int peek_num = 1000;
  float *peek_features = new float[peek_num * dimension];
  assert(0 == queue->Peek(peek_features, dimension, peek_num));
  for (int i = 0; i < peek_num; i++) {
    float *expect = buildVector(dimension, i);
    const float *peek_feature = peek_features + i * dimension;
    cerr << "float array equal, i=" << i << ", feature=[" << peek_feature[0]
         << ", " << peek_feature[1] << ", " << peek_feature[2] << "]"
         << ", expect=[" << expect[0] << ", " << expect[1] << ", " << expect[2]
         << "]" << endl;
    assert(floatArrayEquals(expect, dimension, peek_feature, dimension));
    delete[] expect;
  }
  assert(doc_num == queue->PeekSize());
  queue->MovePeekIndex(peek_num);
  assert(doc_num - peek_num == queue->PeekSize());
  assert(doc_num == queue->Size());
  delete[] peek_features;
  peek_features = nullptr;

  int poll_num = 500;
  queue->MovePollIndex(poll_num);
  assert(doc_num - poll_num == queue->Size());

  for (int i = 0; i < poll_num; i++) {
    float *feature = buildVector(dimension, i + doc_num);
    // cerr << "add i=" << i << endl;
    assert(0 == queue->Add(feature, dimension, -1));
    delete[] feature;
  }

  assert(doc_num - peek_num + poll_num == queue->PeekSize());
  assert(doc_num == queue->Size());

  int peek_num_2 = doc_num - peek_num + poll_num;
  peek_features = new float[peek_num_2 * dimension];
  assert(0 == queue->Peek(peek_features, dimension, peek_num_2));
  for (int i = 0; i < peek_num_2; i++) {
    float *expect = buildVector(dimension, i + peek_num);
    const float *peek_feature = peek_features + i * dimension;
    cerr << "float array equal, i=" << i << ", feature=[" << peek_feature[0]
         << ", " << peek_feature[1] << ", " << peek_feature[2] << "]"
         << ", expect=[" << expect[0] << ", " << expect[1] << ", " << expect[2]
         << "]" << endl;
    assert(floatArrayEquals(expect, dimension, peek_feature, dimension));
    delete[] expect;
  }
  queue->MovePeekIndex(peek_num_2);
  assert(0 == queue->PeekSize());
  assert(doc_num == queue->Size());
  delete queue;
}

void TestFeatureBufferQueueRandRead() {
  int dimension = 512;
  int resident_buffer_size = 1000000;
  int max_buffer_size = resident_buffer_size * 2;
  int read_times = 1000000;

  VectorBufferQueue *queue =
      new VectorBufferQueue(max_buffer_size, resident_buffer_size, dimension);
  assert(0 == queue->Init(0));
  assert(0 == queue->PeekSize());
  assert(0 == queue->Size());

  int doc_num = resident_buffer_size;
  for (int i = 0; i < doc_num; i++) {
    float *feature = buildVector(dimension, i);
    // cerr << "add i=" << i << endl;
    assert(0 == queue->Add(feature, dimension, -1));
    delete[] feature;
  }
  assert(doc_num == queue->PeekSize());
  assert(doc_num == queue->Size());

  std::srand(std::time(nullptr));
  float *feature = new float[dimension];
  double begin = utils::getmillisecs();
  for (int i = 0; i < read_times; i++) {
    int id = std::rand();
    id = id % resident_buffer_size;
    assert(0 == queue->GetVector(id, feature, dimension));
    cerr << "i=" << i << ", id=" << id << ", feature[0]=" << feature[0] << endl;
  }
  cerr << "rand read finished, times=" << read_times
       << ", cost=" << utils::getmillisecs() - begin << "ms" << endl;
}

void AddFunc(VectorBufferQueue *qu, int check_num, int dim) {
  cerr << "****AddFunc: check num=" << check_num << ", dimension=" << dim
       << endl;
  srand(time(NULL));
  for (int i = 0; i < check_num; i++) {
    float *v = buildVector(dim, i);
    assert(0 == qu->Add(v, dim, -1));
    int wait = rand() % 10;
    cout << "****AddFunc: add vector, i=" << i << ", size=" << qu->Size()
         << "peek size=" << qu->PeekSize() << ", next wait=" << wait << "ms"
         << endl;
    delete[] v;
    std::this_thread::sleep_for(std::chrono::milliseconds(wait));
  }
}

int Flush(VectorBufferQueue *feature_buffer_queue, int check_num,
          int dimension, int flush_batch, int resident_buffer_size) {
  cerr << "****FlushFunc: check num=" << check_num
       << ", dimension=" << dimension << ", flush batch=" << flush_batch
       << ", resident=" << resident_buffer_size << endl;
  int checked = 0;
  float *flush_batch_features = new float[flush_batch * dimension];
  while (checked < check_num) {
    try {
      int peek_size = feature_buffer_queue->PeekSize();
      int peek_num = peek_size > flush_batch ? flush_batch : peek_size;
      if (peek_num > 0) {
        cerr << "peek number=" << peek_num << endl;
        // error handle
        assert(0 == feature_buffer_queue->Peek(flush_batch_features, dimension,
                                               peek_num));
        for (int i = 0; i < peek_num; i++) {
          float *expect = buildVector(dimension, checked);
          const float *peek_feature = flush_batch_features + i * dimension;
          cerr << "float array equal, checked=" << checked << ", feature=["
               << peek_feature[0] << ", " << peek_feature[1] << ", "
               << peek_feature[2] << "]"
               << ", expect=[" << expect[0] << ", " << expect[1] << ", "
               << expect[2] << "]" << endl;
          assert(floatArrayEquals(expect, dimension, peek_feature, dimension));
          checked++;
          delete[] expect;
        }

        cerr << "flush one batch features to disk success! peek size="
             << peek_size << ", peek number=" << peek_num
             << ", max flushed feature id=" << checked << endl;
        feature_buffer_queue->MovePeekIndex(peek_num);

      } else {
        cerr << "no feature need to flush, buffer queue size="
             << feature_buffer_queue->Size() << endl;
      }

      int size = feature_buffer_queue->Size();
      if (size > resident_buffer_size) {
        feature_buffer_queue->MovePollIndex(size - resident_buffer_size);
      }

      std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
    catch (const std::exception &e) {
      cerr << "Flush exception: " << e.what() << endl;
      std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }
  }
  return 0;
}

void TestFeatureBufferQueueTwoThreads() {
  int dimension = 512;
  int resident_buffer_size = 100;
  int max_buffer_size = resident_buffer_size * 2;
  int check_num = 10000;
  int flush_batch = 20;

  VectorBufferQueue *queue =
      new VectorBufferQueue(max_buffer_size, resident_buffer_size, dimension);
  assert(0 == queue->Init(0));
  std::thread addThread(AddFunc, queue, check_num, dimension);
  std::thread pollThread(Flush, queue, check_num, dimension, flush_batch,
                         resident_buffer_size);
  addThread.join();
  pollThread.join();
  delete queue;
  cout << "----case:testAddPollTwoThreads()---: success!" << endl;
}

int main(int argc, char *argv[]) {
  // TestFileMapper();
  // TestFileMapperLoad();
  // TestFileMapperRandRead();
  // TestMemoryDisckRawFeature();
  // TestFeatureBufferQueue();
  // TestFeatureBufferQueueTwoThreads();

  // int max_size = (int)std::strtol(argv[1], NULL, 10);
  // int read_times = (int)std::strtol(argv[2], NULL, 10);
  // TestMemoryDiskRawFeatureRandomGet(max_size, read_times);
  TestFeatureBufferQueueRandRead();
}
