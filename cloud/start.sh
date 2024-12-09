#!/bin/bash

if [ -f /sys/fs/cgroup/cpu/cpu.cfs_quota_us ]; then
    CPUS=$(cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us) && [ -n $CPUS ] && [ $CPUS -gt 0 ] && CPUS=$(expr $CPUS / 100000) && echo $CPUS && export OMP_NUM_THREADS=$CPUS
fi

if [ -z "$1" ]; then
    echo "start args is empty"
fi

export LD_LIBRARY_PATH=/vearch/lib/:$LD_LIBRARY_PATH
/vearch/bin/vearch -conf /vearch/config.toml $1
