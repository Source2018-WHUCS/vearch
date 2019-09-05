#!/bin/bash

path=`pwd`

# install pytorch and torchvision
conda install pytorch torchvision cudatoolkit=9.2 -c pytorch

# download mmdetection
cd model
git clone https://github.com/open-mmlab/mmdetection.git
cd mmdetection
python setup.py develop


# download weight
cd $path
cd model/image_detect
sh weight.sh

# install package
isexist(){
    pymod=$1
    if python -c "import $pymod" > /dev/null 2>&1
    then
        echo "found ${pymod}"
    else
        echo "not found ${pymod},begin install $2"
        pip install   $2
    fi
}
error_exit(){
    echo ""
    echo "$1"
    exit 1
}
isexist "requests" "requests"  2>/dev/null || error_exit  "install requests failed"
isexist "tornado" "tornado==6.0.2"  2>/dev/null || error_exit  "install tornado==6.0.2 failed"
isexist "shortuuid" "shortuuid"   2>/dev/null || error_exit  "install shortuuid failed"
isexist "cv2" "opencv-python"  2>/dev/null || error_exit "install opencv-python failed"
isexist "sklearn" "sklearn"  2>/dev/null || error_exit  "install sklearn failed"

echo "Begin torndao service!!!"
python main.py
