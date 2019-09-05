#!/usr/bin/env bash
mkdir /app

#for great wall
curl http://mirrors.aliyun.com/repo/Centos-7.repo > /etc/yum.repos.d/CentOS-Base.repo

yum update
yum install -y gcc gcc-c++ make automake git

cd /app
cp /vearch/cloud/app/cmake-3.12.2.tar.gz /app/cmake-3.12.2.tar.gz
tar -xzf cmake-3.12.2.tar.gz
cd /app/cmake-3.12.2
./bootstrap
gmake
gmake install

cd /vearch/cloud/app/faiss
./configure --without-cuda && make install

cd /app
# unzip go
cp /vearch/cloud/app/go1.12.7.linux-amd64.tar.gz /app/go1.12.7.linux-amd64.tar.gz
tar -xzf go1.12.7.linux-amd64.tar.gz
#add env
export GOROOT=/app/go
export PATH=$PATH:$GOROOT/bin


# to compile
cd /vearch/build
mkdir -p /root/go
ln -s /vearch/vendor/ /root/go/src
mkdir -p /app/go/src/github.com/vearch
ln -s /vearch/ /app/go/src/github.com/vearch/vearch
./build.sh

# del make file
rm -rf /vearch/build/bin
rm -rf /vearch/build/gamma_build

