#!/usr/bin/env bash
mkdir /app

yum install -y gcc gcc-c++ make automake git

cd app
cp /vearch/cloud/app/cmake-3.12.2.tar.gz /app/cmake-3.12.2.tar.gz
tar -xzvf cmake-3.12.2.tar.gz
cd /app/cmake-3.12.2
./bootstrap
gmake
gmake install



cd /app
# unzip go
cp /vearch/cloud/app/go1.12.7.linux-amd64.tar.gz /app/go1.12.7.linux-amd64.tar.gz
tar -xzvf go1.12.7.linux-amd64.tar.gz
#add env
export GOROOT=/app/go
export PATH=$PATH:$GOROOT/bin


# to compile
cd /vearch/build
#mkdir -p /root/go/src
#ln -s /vearch/vendor/ /root/go/src
mkdir -p /app/go/src/github.com/vearch
ln -s /vearch/ /app/go/src/github.com/vearch/vearch
./build.sh

# del make file
rm -rf /vearch/build/bin
rm -rf /vearch/build/gamma_build

