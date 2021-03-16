# Vearch source compilation and deployment

## Source code compilation

### Dependent Environment 

   1. CentOS, Ubuntu and Mac OS are all OK (recommend CentOS >= 7.2)
   2. go >= 1.11.2 required
   3. gcc >= 5 required
   4. cmake >= 3.17 required
   5. OpenBLAS
   6. [RocksDB](https://github.com/facebook/rocksdb) == 6.2.2 ***(optional)***. Rocksdb does not require you to install it, the script installs automatically. But you need to manually install the dependencies of rocksdb. Please refer to the installation method: https://github.com/facebook/rocksdb/blob/master/INSTALL.md
   7. [zfp](https://github.com/LLNL/zfp) == v0.5.5, You don't need to install it, the script installs automatically.
   8. CUDA >= 9.0, if you want GPU support. 
### Compile
   * Enter the `GOPATH` directory, `cd $GOPATH/src` `mkdir -p github.com/vearch` `cd github.com/vearch`

   * Download the source code: `git clone https://github.com/vearch/vearch.git` ($vearch denotes the absolute path of vearch code)

   * Download the source code of subprojects gamma: `git submodule init`  `git submodule update`

   * To add GPU Index support : change `BUILD_WITH_GPU` from `"off"` to `"on"` in `$vearch/engine/CMakeLists.txt` 

   * Compile vearch
      1. `cd vearch/build`
      
      2. `sh build.sh`
      
         when `vearch` file generated, it is ok.
      
## Deploy
   Before run vearch, you shuld set `LD_LIBRARY_PATH`, Ensure that system can find gamma dynamic libraries (like $vearch/ps/engine/gammacb/lib/lib) .
   ### 1 Local Model
   * generate config file conf.toml
     
```
[global]
    # the name will validate join cluster by same name
    name = "vearch"
    # you data save to disk path ,If you are in a production environment, You'd better set absolute paths
    data = ["datas/"]
    # log path , If you are in a production environment, You'd better set absolute paths
    log = "logs/"
    # default log type for any model
    level = "debug"
    # master <-> ps <-> router will use this key to send or receive data
    signkey = "vearch"
    skip_auth = true

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
   * start

````
./vearch -conf conf.toml all
````

   ### 2 Cluster Model
   > vearch has three module: `ps`(PartitionServer) , `master`, `router`, run `./vearch -f conf.toml ps/router/master` start ps/router/master module

   > Now we have five machine, two master, two ps and one router

* master
    * 192.168.1.1
    * 192.168.1.2
* ps
    * 192.168.1.3
    * 192.168.1.4
* router
    * 192.168.1.5
* generate config file conf.toml

````
[global]
    name = "vearch"
    data = ["datas/"]
    log = "logs/"
    level = "debug"
    signkey = "vearch"
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
* on 192.168.1.1 , 192.168.1.2  run master

````
./vearch -conf conf.toml master
````

* on 192.168.1.3 , 192.168.1.4 run ps

````
./vearch -conf conf.toml ps
````

* on 192.168.1.5 run router

````
./vearch -conf conf.toml router
````