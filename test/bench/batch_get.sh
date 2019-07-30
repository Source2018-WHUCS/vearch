#!/bin/bash 

for((i=1;i<=1000000;i++));
do

date "+%Y-%m-%d %H:%M:%S.%N"
curl -X GET http://cb:1234@127.0.0.1:9001/db1/space1/_search?size=0
echo
date "+%Y-%m-%d %H:%M:%S.%N"

sleep 5
echo

done

