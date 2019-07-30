CMD = -O3 -g -DNDEBUG
CXXFLAGS = $(CMD) -fno-strict-aliasing -Wall -fPIC
TOPDIR=../..
THIRD_PARTY=$(TOPDIR)/3rdparty
LIBS=-L /usr/lib64 -lpthread ./libcache.a \
	 $(THIRD_PARTY)/openssl/lib/libssl.a \
	 $(THIRD_PARTY)/crypto/lib/libcrypto.a
INCLUDES=-I. -I/usr/local/include -I/usr/include -I$(TOPDIR)
COMMON_OBJS= cache_map.o mem_cache.o thread_lock.o

BINS = libcache.a test test2
all : $(BINS)

libcache.a : $(COMMON_OBJS)
	ar -r $@ $^

test: test.o 
	make libcache.a
	$(CXX) $^ -o $@ $(LIBS) 

test2: test2.o
	make libcache.a	
	$(CXX) $^ -o $@ $(LIBS) 

.cpp.o :
	$(CXX) $(CXXFLAGS) -c $^ $(INCLUDES) 

.cc.o :
	$(CXX) $(CXXFLAGS) -c $^ $(INCLUDES) 

debug:
	make CMD=-g

gprof:
	make CMD="-g -pg"

.PHONY : clean
clean:
	rm -f *.o
	@for i in $(BINS);\
	do \
		echo "rm -f $$i"; \
		rm -f $$i; \
	done
