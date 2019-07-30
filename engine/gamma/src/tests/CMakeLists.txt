FILE(GLOB srcs ${CMAKE_CURRENT_SOURCE_DIR}/*.cc)

ENABLE_TESTING()
# Build each source file independently
INCLUDE_DIRECTORIES(../)

# gtest
FIND_PACKAGE(GTest REQUIRED)
INCLUDE_DIRECTORIES(${GTEST_INCLUDE_DIRS})

LINK_DIRECTORIES(../)

FOREACH(source ${srcs})
    GET_FILENAME_COMPONENT(name ${source} NAME_WE)

    # target
    ADD_EXECUTABLE(${name} ${source})
    TARGET_LINK_LIBRARIES(${name} gamma ${GTEST_BOTH_LIBRARIES})

    # Install
    INSTALL(TARGETS ${name} DESTINATION test)
    
    # ADD_TEST(NAME ${name} COMMAND ${name})
    # ADD_CUSTOM_COMMAND(TARGET ${name}
    #                    POST_BUILD 
    #                    COMMAND ${name}
    #                    WORKING_DIRECTORY ${CMAKE_CURRENT_BINARY_DIR}
    #                    COMMENT "Running ${name}" VERBATIM)
ENDFOREACH(source)