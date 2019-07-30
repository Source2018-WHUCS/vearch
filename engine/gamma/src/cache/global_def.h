#ifndef _GLOBAL_DEF_H_
#define _GLOBAL_DEF_H_

#define CONV_UTF8 0

#include <stdio.h>
#include <string.h>
#include <stdlib.h>
#include <vector>
#include <new>
#include <string>
#include <limits.h>

using std::string;
using std::vector;

typedef std::vector<std::string> StrVec;
typedef std::vector<int> IntVec;

#if CRC64_KEYID
#include <stdint.h>
typedef uint64_t KEYID_TYPE;
#else
typedef int KEYID_TYPE;
#endif

//�ͷ��ڴ棬 ����Ұָ��
#define SAFE_FREE(p)   if (p != NULL) {free (p); p = NULL;}
#define SAFE_DELETE(p) if (p != NULL) {delete (p); p = NULL;}
#define SAFE_DELETE_ARRAY(p) if (p != NULL) {delete[] (p); p = NULL;}

#if defined(__GNUC__) && __GNUC__ >= 4
#define likely(x)   (__builtin_expect((x), 1))
#define unlikely(x) (__builtin_expect((x), 0))
#else
#define likely(x)   (x)
#define unlikely(x) (x)
#endif

#define HAS_CITY_STORE_POS 59
#define CITY_STORE_BIT (1L << HAS_CITY_STORE_POS)
#define ALL_IN_STOCK 0x5000000FFFFFFFFE

#ifndef IN
#define IN
#endif

#ifndef OUT
#define OUT
#endif

#ifndef INOUT
#define INOUT
#endif

#endif
