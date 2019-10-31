# Raw Vector User Guide for Gamma Engine


This module is responsible for storing raw vectors. RawVector is the base class and has multiple sub-classes depending on the implementations. Currently, the implementations base on mmap and RocksDB are supported.

## Mmap Raw Vector

When the MmapRawVector is initialized, a buffer queue is created and a disk file is mapped to the virtual address space in memory via mmap. When a vector is inserted, it is first inserted into the buffer queue and then written asynchronously to the disk file by a flushing thread. When a vector is read, it is first fetched from the buffer queue, if it does not exist, and then from the address space mapped by mmap.

## RocksDB Raw Vector

RocksDBRawVector integrates RocksDB and it creates a database in RocksDB. Vectors are inserted directly into the database, and the key inserted is the vector id, and value is the vector itself. When read, it is also read directly from the database through a vector id.


