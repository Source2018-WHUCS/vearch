# vearch编译部署流程

[TOC]

## 一、编译

#### 1、依赖环境

   1. Linux 系统(推荐CentOS 7.2以上)，支持cmake、make 命令
   2. Go版本1.11.2以上
   3. gcc版本5以上
   4. [Faiss](https://github.com/facebookresearch/faiss)

#### 2、编译
   * 下载源代码: git clone https://xxxxxx/vearch.git (后续使用$vearch 代表vearch目录绝对路径)
   * 编译gamma
       1. `cd $vearch/engine/gamma/src`
       2. `mkdir build && cd build`
       3. `export Faiss_HOME=faiss安装路径`
       4. `cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=gamma/ ..`
       5. `make && make install`
      
   * 编译vearch
      1. `cd $vearch`
      2. `export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:$vearch/engine/gammacb/lib/lib`
      3. `export Faiss_HOME=faiss安装路径`
      4. `go build -a --tags=vector -o  baudengine`
      生成`baudengine`文件表示编译成功
       
## 二、部署
   #### 1、单机模式
   * 生成配置文件conf.toml
      
```
[global]
    # the name will validate join cluster by same name
    name = "baudengine"
    # you data save to disk path ,If you are in a production environment, You'd better set absolute paths
    data = ["datas/"]
    # log path , If you are in a production environment, You'd better set absolute paths
    log = "logs/"
    # default log type for any model
    level = "debug"
    # master <-> ps <-> router will use this key to send or receive data
    signkey = "baudengine"

# if you are master you'd better set all config for router and ps and router and ps use default config it so cool
[[masters]]
    # name machine name for cluster
    name = "m1"
    # ip or domain
    address = "127.0.0.1"
    # api port for http server
    api_port = 8817
    # port for etcd server
    etcd_port = 2378
    # listen_peer_urls List of comma separated URLs to listen on for peer traffic.
    # advertise_peer_urls List of this member's peer URLs to advertise to the rest of the cluster. The URLs needed to be a comma-separated list.
    etcd_peer_port = 2390
    # List of this member's client URLs to advertise to the public.
    # The URLs needed to be a comma-separated list.
    # advertise_client_urls AND listen_client_urls
    etcd_client_port = 2370
    skip_auth = true

[router]
    # port for server
    port = 9001
    # skip auth for client visit data
    skip_auth = true

[ps]
    # port for server
    rpc_port = 8081
    # raft config begin
    raft_heartbeat_port = 8898
    raft_replicate_port = 8899
    heartbeat-interval = 200 #ms
    raft_retain_logs = 10000
    raft_replica_concurrency = 1
    raft_snap_concurrency = 1 
```
   * 启动

````
./baudengine -conf conf.toml
````
   
   #### 2、集群模式
   > vearch 有三个模块: `ps`(PartitionServer) , `master`, `router`, 运行 `./baudengine -f conf.toml ps/router/master` 启动指定模块

   > 以5台机器为例: 2台作为master, 2台作为ps, 1台作为router

* master
    * 192.168.1.1
    * 192.168.1.2
* ps
    * 192.168.1.3
    * 192.168.1.4
* router
    * 192.168.1.5
* 生成配置文件conf.toml

````
[global]
    name = "baudengine"
    data = ["datas/"]
    log = "logs/"
    level = "debug"
    signkey = "baudengine"
    skip_auth = true

# if you are master you'd better set all config for router and ps and router and ps use default config it so cool
[[masters]]
    name = "m1"
    address = "192.168.1.1"
    api_port = 8817
    etcd_port = 2378
    etcd_peer_port = 2390
    etcd_client_port = 2370
[[masters]]
    name = "m2"
    address = "192.168.1.2"
    api_port = 8817
    etcd_port = 2378
    etcd_peer_port = 2390
    etcd_client_port = 2370
[router]
    port = 9001
    skip_auth = true
[ps]
    rpc_port = 8081
    raft_heartbeat_port = 8898
    raft_replicate_port = 8899
    heartbeat-interval = 200 #ms
    raft_retain_logs = 10000
    raft_replica_concurrency = 1
    raft_snap_concurrency = 1
````
* 在 192.168.1.1 , 192.168.1.2 运行master

````
./baudengine -conf conf.toml master
````

* 在 192.168.1.3 , 192.168.1.4 运行ps

````
./baudengine -conf conf.toml ps
````

* 在192.168.1.5 运行router

````
./baudengine -conf conf.toml router
````

