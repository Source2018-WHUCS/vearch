#include "mem_cache.h"
#include <stdlib.h>
#include <time.h>

MemCache _cache;

void *thread_put(void *para) {
  int nTotal = *(int *) para;
  char pKey[64], pVal[64];
  time_t now;
  time(&now);
  unsigned int seed = static_cast<unsigned int>(now);

  for (int i = 0; i < nTotal; i++) {
    int n = rand_r(&seed);
    sprintf(pKey, "%d", n);
    MCElem key(pKey, strlen(pKey));

    sprintf(pVal, "%08d", n);
    MCElem val;
    val.len = strlen(pVal);
    val.data = new(std::nothrow) char[val.len];
    strcpy(val.data, pVal);

    int nRes = 0;
    if ((nRes = _cache.put(key, val)) != 0) {
      fprintf(stderr, "put cache fail, error=%d\n", nRes);
      return NULL;
    }
  }
  return NULL;
}

void *thread_get(void *para) {
  int nTotal = *(int *) para;
  char pKey[64], pVal[64];
  time_t now;
  time(&now);
  unsigned int seed = static_cast<unsigned int>(now);

  for (int i = 0; i < nTotal; i++) {
    int n = rand_r(&seed);
    sprintf(pKey, "%d", n);
    MCElem key(pKey, strlen(pKey));
    sprintf(pVal, "%08d", n);
    MCElem val;

    if (_cache.get(key, val) == 0) {
      if (val.data != NULL) {
        if (strncmp(val.data, pVal, val.len) != 0) {
          fprintf(stderr, "search fail1, id=%d\n", i);
          return NULL;
        }
        delete[] val.data;
      }
    }
  }
  return NULL;
}

int main(int argc, char **argv) {
  if (argc < 2) {
    fprintf(stderr, "%s: cnt\n", argv[0]);
    return -1;
  }

  const static uint64_t MAX_SIZE = 1 << 30;
  const static uint64_t MAX_ITEM = 1000000;
  if (0 != _cache.init(MAX_SIZE, MAX_ITEM)) {
    return -1;
  }
  int nTotal = strtol(argv[1], NULL, 10);

  int thread_num = 10;
  pthread_t thread_put_ids[thread_num];
  for (int i = 0; i < thread_num; i++) {
    if (pthread_create(&thread_put_ids[i], NULL, thread_put, &nTotal)) {
      return -2;
    }
  }

  pthread_t thread_get_ids[thread_num];
  for (int i = 0; i < thread_num; i++) {
    if (pthread_create(&thread_get_ids[i], NULL, thread_get, &nTotal)) {
      return -3;
    }
  }

  for (int i = 0; i < thread_num; i++) {
    pthread_join(thread_put_ids[i], NULL);
  }
  for (int i = 0; i < thread_num; i++) {
    pthread_join(thread_get_ids[i], NULL);
  }

  stringstream output;
  _cache.getStat(output);
  printf("%s", output.str().c_str());
  return 0;
}

