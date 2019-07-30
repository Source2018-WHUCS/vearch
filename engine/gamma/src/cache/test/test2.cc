#include "mem_cache.h"
#include <stdlib.h>
#include <time.h>

MemCache _cache;

int put(char *key, char *value) {
  MCElem mckey(key, strlen(key));
  MCElem mcval;

  mcval.len = strlen(value);
  mcval.data = new(std::nothrow) char[mcval.len];
  strcpy(mcval.data, value);

  int nRes = 0;
  if ((nRes = _cache.put(mckey, mcval)) != 0) {
    fprintf(stderr, "put cache fail, error=%d\n", nRes);
    return -1;
  }
  return 0;
}

char *get(char *key) {
  MCElem mckey(key, strlen(key));
  MCElem mcval;

  if (_cache.get(mckey, mcval) != 0) {
    return NULL;
  }
  return mcval.data;
}

int main(int argc, char **argv) {
  const static uint64_t MAX_SIZE = 1 << 20;
  const static uint64_t MAX_ITEM = 1000;
  int ret = 0;
  if ((ret = _cache.init(MAX_SIZE, MAX_ITEM)) != 0) {
    fprintf(stderr, "init cache fail. err=%d\n", ret);
    return -1;
  }

  char key1[] = "key";
  char val1[] = "val1";
  put(key1, val1);

  char *result = get(key1);
  if (result != NULL) {
    printf("from cache. key=%s, value=%s\n", key1, result);
    delete[] result;
  } else {
    fprintf(stderr, "get %s from cache fail\n", key1);
  }

  char key2[] = "key";
  char val2[] = "val2";
  put(key2, val2);

  result = get(key2);
  if (result != NULL) {
    printf("from cache. key=%s, value=%s\n", key2, result);
    delete[] result;
  } else {
    fprintf(stderr, "get %s from cache fail\n", key2);
  }

  return 0;
}

