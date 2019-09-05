SET(Glog_INCLUDE_SEARCH_PATHS
   /usr/include
   /usr/include/glog
   /usr/local/include
   /usr/local/include/glog
   $ENV{Glog_HOME}
   $ENV{Glog_HOME}/include
)

SET(Glog_LIB_SEARCH_PATHS
    /lib/
    /lib64/
    /usr/lib
    /usr/lib64
    /usr/local/lib
    /usr/local/lib64
    $ENV{Glog}/lib
    $ENV{Glog_HOME}
    $ENV{Glog_HOME}/lib
 )

FIND_PATH(Glog_INCLUDE_DIR NAMES logging.h PATHS ${Glog_INCLUDE_SEARCH_PATHS})
FIND_LIBRARY(Glog_LIB NAMES glog PATHS ${Glog_LIB_SEARCH_PATHS})

SET(Glog_FOUND ON)

# Check include files
IF(NOT Glog_INCLUDE_DIR)
    SET(Glog_FOUND OFF)
    MESSAGE(STATUS "Could not find Glog include. Turning Glog_FOUND off")
ENDIF()

# Check libraries
IF(NOT Glog_LIB)
    SET(Glog_FOUND OFF)
    MESSAGE(STATUS "Could not find Glog lib. Turning Glog_FOUND off")
ENDIF()

IF (Glog_FOUND)
  IF (NOT Glog_FIND_QUIETLY)
    MESSAGE(STATUS "Found Glog libraries: ${Glog_LIB}")
    MESSAGE(STATUS "Found Glog include: ${Glog_INCLUDE_DIR}")
  ENDIF (NOT Glog_FIND_QUIETLY)
ELSE (Glog_FOUND)
  IF (Glog_FIND_REQUIRED)
    MESSAGE(FATAL_ERROR "Could not find Glog, please install Glog or set $Glog_HOME")
  ENDIF (Glog_FIND_REQUIRED)
ENDIF (Glog_FOUND)

MARK_AS_ADVANCED(
    Glog_INCLUDE_DIR
    Glog_LIB
    Glog
)

