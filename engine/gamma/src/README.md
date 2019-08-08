# Gamma 
Gamma is the core vector search engine of VectorBase. It is a high-performance, concurrent vector search engine, and supports real time indexing vectors and scalars without lock. Differently from general vector search engine, Gamma can store and index a document which contains scalars and vectors, and provides the ablity of quickly indexing and filtering by numeric scalar fields. The work of design and implementation of real time indexing has been publish in the paper[The Design and Implementation of a Real Time Visual Search System On JD E-commerce Platform] of ACM Middleware 2018. It is developed by TIG Team of JD.COM.
As for the part of similarity search of vectors in Gamma, it is mainly implemented based on faiss which is an open source library developed by Facebook AI Research. Besides faiss, it can easily support other approximate nearest neighbor search(ANN) algorithms or libraries. 

## Requirements 
* [Faiss](https://github.com/facebookresearch/faiss)

## Installation
1. `mkdir build`
2. `cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=gamma/..`
3. `make`
4. `make install`

## Issue Report

 
## License
