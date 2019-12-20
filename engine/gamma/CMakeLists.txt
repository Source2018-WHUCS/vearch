#--------------------------------------------
#    GAMMA ENGINE 
#--------------------------------------------
project(gamma_engine C CXX)
cmake_minimum_required(VERSION 3.0)

list(APPEND CMAKE_MODULE_PATH ${PROJECT_SOURCE_DIR}/cmake/Modules)

find_package(Faiss REQUIRED)

option(BUILD_TEST "Build tests" off)
option(BUILD_GTEST "Build gtests" on)
option(BUILD_TOOLS "Build tools" off)

#ENV VARs
set(THIRDPARTY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
set(BTREE ${CMAKE_CURRENT_SOURCE_DIR}/third_party/btree)
set(CJSON ${CMAKE_CURRENT_SOURCE_DIR}/third_party/cjson)
set(EASYLOGGINGPP ${CMAKE_CURRENT_SOURCE_DIR}/third_party/easyloggingpp)
set(HASHTABLE ${CMAKE_CURRENT_SOURCE_DIR}/third_party/libcuckoo)
set(UTIL ${CMAKE_CURRENT_SOURCE_DIR}/util)
set(GAMMA_SEARCH ${CMAKE_CURRENT_SOURCE_DIR}/search)
set(GAMMA_INDEX ${CMAKE_CURRENT_SOURCE_DIR}/index)
set(GAMMA_VECTOR ${CMAKE_CURRENT_SOURCE_DIR}/vector)
set(GAMMA_REALTIME ${CMAKE_CURRENT_SOURCE_DIR}/realtime)
set(GAMMA_PROFILE ${CMAKE_CURRENT_SOURCE_DIR}/profile)
set(GAMMA_C_API ${CMAKE_CURRENT_SOURCE_DIR}/c_api)

set(CMAKE_CXX_FLAGS_DEBUG "$ENV{CXXFLAGS} -std=c++11 -mavx2 -msse4 -mpopcnt -fopenmp -D_FILE_OFFSET_BITS=64 -D_LARGE_FILE -DOPEN_CORE -O0 -w -g3 -gdwarf-2")
set(CMAKE_CXX_FLAGS_RELEASE "$ENV{CXXFLAGS} -std=c++11 -fPIC -m64 -Wall -O3 -mavx2 -msse4 -mpopcnt -fopenmp -D_FILE_OFFSET_BITS=64 -D_LARGE_FILE -Werror=narrowing -Wno-deprecated")

if (DEFINED ENV{ROCKSDB_HOME})
    message(STATUS "RocksDB home is set=$ENV{ROCKSDB_HOME}")
    add_definitions(-DWITH_ROCKSDB)
    find_package(RocksDB REQUIRED)
else (DEFINED ENV{ROCKSDB_HOME})
    message(STATUS "RocksDB home isn't set, so store_type=RocksDB is not supported! ")
endif (DEFINED ENV{ROCKSDB_HOME})

# install FlatBuffers
if (NOT EXISTS "${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers")
    execute_process(COMMAND rm -rf flatbuffers WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    execute_process(COMMAND wget https://github.com/google/flatbuffers/archive/v1.11.0.tar.gz WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    execute_process(COMMAND tar xf v1.11.0.tar.gz WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    execute_process(COMMAND rm -rf v1.11.0.tar.gz WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
    execute_process(COMMAND cmake -DCMAKE_INSTALL_INCLUDEDIR=${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers/include 
    -DCMAKE_INSTALL_LIBDIR=${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers/lib 
    -DCMAKE_INSTALL_BINDIR=${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers/bin . WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers-1.11.0)
    execute_process(COMMAND make WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers-1.11.0)
    execute_process(COMMAND make install WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers-1.11.0)
    execute_process(COMMAND rm -rf flatbuffers-1.11.0 WORKING_DIRECTORY ${CMAKE_CURRENT_SOURCE_DIR}/third_party)
endif ()

#INCLUDE DIR
include_directories(
    ${THIRDPARTY}
    ${BTREE}
    ${CJSON}
    ${EASYLOGGINGPP}
    ${HASHTABLE}
    ${UTIL}
    ${GAMMA_PROFILE}
    ${GAMMA_SEARCH}
    ${GAMMA_INDEX}
    ${GAMMA_VECTOR}
    ${GAMMA_REALTIME}
    ${GAMMA_C_API}
    ${Faiss_INCLUDE_DIR}
    ${RocksDB_INCLUDE_DIR}
    ${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers/include
)

link_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/third_party/flatbuffers/lib/
)

#LINK LIBRARY
set(LIBRARIES
    ${Faiss_LIB}
    ${RocksDB_LIB}
    flatbuffers
)

#ADDITIONAL SOURCE DIR
aux_source_directory(${THIRDPARTY} DIR_SRCS)
aux_source_directory(${BTREE} DIR_SRCS)
aux_source_directory(${CJSON} DIR_SRCS)
aux_source_directory(${EASYLOGGINGPP} DIR_SRCS)
aux_source_directory(${UTIL} DIR_SRCS)
aux_source_directory(${GAMMA_SEARCH} DIR_SRCS)
aux_source_directory(${GAMMA_INDEX} DIR_SRCS)
aux_source_directory(${GAMMA_VECTOR} DIR_SRCS)
aux_source_directory(${GAMMA_REALTIME} DIR_SRCS)
aux_source_directory(${GAMMA_PROFILE} DIR_SRCS)
aux_source_directory(${GAMMA_C_API} DIR_SRCS)

set(LIB_SRCS ${DIR_SRCS})

# ADD DEFINITION
add_definitions(-DHAVE_NETINET_IN_H)
add_definitions(-DFINTEGER=int)

if (CMAKE_BUILD_TYPE STREQUAL "Debug")
    message(STATUS "Debug Mode")
    message(STATUS "Flags:" ${CMAKE_CXX_FLAGS_DEBUG})
    add_definitions(-DDEBUG_) 
else (CMAKE_BUILD_TYPE STREQUAL "Debug")
    set(CMAKE_BUILD_TYPE "Release" CACHE STRING "build type" FORCE)
    message(STATUS "Release Mode")
    message(STATUS "Flags:" ${CMAKE_CXX_FLAGS_RELEASE})
endif (CMAKE_BUILD_TYPE STREQUAL "Debug")

if (PERFORMANCE_TESTING STREQUAL "ON")
    message(STATUS "With preformance testing")
    add_definitions(-DPERFORMANCE_TESTING) 
endif (PERFORMANCE_TESTING STREQUAL "ON")

add_library(gamma SHARED ${LIB_SRCS})

target_link_libraries(gamma ${LIBRARIES})

set_target_properties(gamma PROPERTIES VERSION 0.1 SOVERSION 0.1)

install(TARGETS gamma
	LIBRARY	DESTINATION	lib 
	ARCHIVE	DESTINATION	lib)

install(FILES ${GAMMA_C_API}/gamma_api.h DESTINATION include)

if (BUILD_TEST)
    add_subdirectory(${CMAKE_CURRENT_SOURCE_DIR}/tests)
endif (BUILD_TEST)

if (BUILD_TOOLS)
    add_subdirectory(${CMAKE_CURRENT_SOURCE_DIR}/tools)
endif (BUILD_TOOLS)
