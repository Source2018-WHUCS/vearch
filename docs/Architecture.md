# Architecture of Vearch

![arc](docs/img/VearchArch.jpg)

* Data Model

  space, documents, vectors, scalars

* Components

  `Master`, `Router` and `PartitionServer` 

* Master 

  Responsible for schema mananagement, cluster-level metadata, and resource coordination. 
  
* Router

  Provides RESTful API: `create`  , `delete`  `search` and `update` ; request routing, and result merging. 

* PartitionServer (PS)

  Hosts document partitions with raft-based replication.

  Gamma`is the core vector search engine. It provides the ability of storing, indexing and retrieving the vectors and scalars.
