#----------------------------------------------------------------
# Generated CMake target import file.
#----------------------------------------------------------------

# Commands may need to know the format version.
set(CMAKE_IMPORT_FILE_VERSION 1)

# Import target "flatbuffers::flatbuffers" for configuration ""
set_property(TARGET flatbuffers::flatbuffers APPEND PROPERTY IMPORTED_CONFIGURATIONS NOCONFIG)
set_target_properties(flatbuffers::flatbuffers PROPERTIES
  IMPORTED_LINK_INTERFACE_LANGUAGES_NOCONFIG "CXX"
  IMPORTED_LOCATION_NOCONFIG "/env/app/go/src/github.com/vearch/vearch/engine/gamma/third_party/flatbuffers/lib/libflatbuffers.a"
  )

list(APPEND _IMPORT_CHECK_TARGETS flatbuffers::flatbuffers )
list(APPEND _IMPORT_CHECK_FILES_FOR_flatbuffers::flatbuffers "/env/app/go/src/github.com/vearch/vearch/engine/gamma/third_party/flatbuffers/lib/libflatbuffers.a" )

# Commands beyond this point should not need to know the version.
set(CMAKE_IMPORT_FILE_VERSION)
