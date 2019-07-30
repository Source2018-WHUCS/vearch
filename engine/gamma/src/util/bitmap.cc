#include "bitmap.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

namespace bitmap {

const static int ALL_SET = ~0;

static const int MASK[SLOT_SIZE] = {
    (int)0x80000000, 0x40000000, 0x20000000, 0x10000000, 0x08000000, 0x04000000,
    0x02000000,      0x01000000, 0x00800000, 0x00400000, 0x00200000, 0x00100000,
    0x00080000,      0x00040000, 0x00020000, 0x00010000, 0x00008000, 0x00004000,
    0x00002000,      0x00001000, 0x00000800, 0x00000400, 0x00000200, 0x00000100,
    0x00000080,      0x00000040, 0x00000020, 0x00000010, 0x00000008, 0x00000004,
    0x00000002,      0x00000001};

char *create(int count, int &len, bool is_set) {
  count = (count + SLOT_SIZE - 1) >> 5;
  len = count * sizeof(int);
  char *bitmap = (char *)malloc(len);
  if (bitmap == NULL)
    return NULL;

  int *data = (int *)bitmap;
  for (int i = 0; i < count; i++) {
    if (is_set) {
      data[i] = ALL_SET;
    } else {
      data[i] = 0;
    }
  }
  return bitmap;
}

int size(int count) {
  count = (count + SLOT_SIZE - 1) >> 5;
  return (count * sizeof(int));
}

void destroy(char *&bitmap) {
  if (bitmap != NULL) {
    free(bitmap);
    bitmap = NULL;
  }
}

bool test(const char *bitmap, int id) {
  int *data = (int *)bitmap;
  return (data[id / SLOT_SIZE] & MASK[id % SLOT_SIZE]) != 0;
}

void set(char *bitmap, int id) {
  int *data = (int *)bitmap;
  data[id / SLOT_SIZE] |= MASK[id % SLOT_SIZE];
}

void set_all(char *bitmap, int count) {
  count = (count + SLOT_SIZE - 1) >> 5;
  int *data = (int *)bitmap;
  for (int i = 0; i < count; i++) {
    data[i] = ALL_SET;
  }
}

void reset(char *bitmap, int id) {
  int *data = (int *)bitmap;
  data[id / SLOT_SIZE] &= ~MASK[id % SLOT_SIZE];
}

void reset_all(char *bitmap, int count) {
  count = (count + SLOT_SIZE - 1) >> 5;
  int *data = (int *)bitmap;
  for (int i = 0; i < count; i++) {
    data[i] = 0;
  }
}

bool is_all_reset(char *bitmap, int count) {
  count = (count + SLOT_SIZE - 1) >> 5;
  int *data = (int *)bitmap;
  for (int i = 0; i < count; i++) {
    if (data[i] != 0)
      return false;
  }
  return true;
}

int mount(char *bitmap, char *data, int size) {
  if (size % sizeof(int) != 0)
    return -1;
  int n = size / sizeof(int);
  int *data2 = (int *)data;
  int *bitmap2 = (int *)bitmap;
  for (int i = 0; i < n; i++) {
    bitmap2[i] = data2[i];
  }
  return 0;
}

int mount(std::vector<int> &bitmap, char *data, int size) {
  if (size % sizeof(int) != 0)
    return -1;
  int n = size / sizeof(int);
  bitmap.resize(n);
  int *data2 = (int *)data;
  for (size_t i = 0; i < bitmap.size(); i++) {
    bitmap[i] = data2[i];
  }
  return 0;
}

void print(const char *bitmap, int count) {
  count = (count + SLOT_SIZE - 1) >> 5;
  const int *data = (int *)bitmap;
  for (int i = 0; i < count; i++) {
    printf("%d ", data[i]);
  }
  printf("\n");
}

} // namespace bitmap

#if 0
int main(int argc, char **argv)
{
    int count = 334, size = 0;
    char *data = bitmap::create(count, size);
    int id = 2;

    if (bitmap::test(data, id)) {
        printf("match0\n");
    }

    bitmap::set(data, id);
    if (bitmap::test(data, id)) {
        printf("match1\n");
    }

    bitmap::reset(data, id);
    if (bitmap::test(data, id)) {
        printf("match2\n");
    }

    char *data2 = bitmap::create(count, size);
    bitmap::set(data2, id);
    bitmap::mount(data, data2, size);
    if (bitmap::test(data, id)) {
        printf("match3\n");
    }

    bitmap::destroy(data);
    bitmap::destroy(data2);
    return 0;
}
#endif
