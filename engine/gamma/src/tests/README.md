# Test User Guide for Gamma Engine

This folder include gamma-engine test cases, you need install GTest to run them.

* test_files

You need to prepare ***profile_100m.txt*** and ***feat_100m_512float.dat***, where ***profile_100m.txt*** is a profile file and ***feat_100m_512float.dat*** is a vector data file.

```flow
st=>start: Start
e=>end: End
op1=>operation: Init engie
op2=>operation: Create table
op3=>operation: Add doc
op4=>operation: Search with add
op5=>operation: Dump engine
op6=>operation: Load engine
op7=>operation: Search
op8=>operation: Close engine

st->op1->op2->op3->op4->op5->op6->op7->op8->e
```
