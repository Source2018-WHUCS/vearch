#!/usr/bin/env bash

echo "Build compile Environment"
./compile_env.sh

echo "Compile Vearch"
./compile.sh

echo "Make Vearch Image"
./build.sh

echo "Start service by all in one model"
docker run -p 8817:8817 -p 9001:9001 -v ./config/config.toml.example:/vearch/config.toml  vearch all

echo "good luck service is ready you can visit http://127.0.0.1:9001 to use it"

curl http://127.0.0.1:9001