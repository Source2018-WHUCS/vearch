# Copyright 2019 The Vearch Authors. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# ==============================================================================

import os
import cv2
import mmcv
import numpy as np
from mmdet.apis import init_detector, inference_detector

class FashionDetect(object):

    def __init__(self):
        local_path = os.path.dirname(os.path.abspath(__file__))
        config_file = os.path.join(local_path, 'faster_rcnn_r50_fpn_1x.py')
        checkpoint_file = os.path.join(local_path, 'faster_rcnn_r50_fpn_1x_20181010-3d1b3351.pth')
        self.model = init_detector(config_file, checkpoint_file, device='cuda:0')

    def detect(self, image):
        result = inference_detector(self.model, image)
        bboxes = np.vstack(result)
        labels = [np.full(bbox.shape[0], i, dtype=np.int32) for i, bbox in enumerate(result)]
        labels = np.concatenate(labels)
        score_thr =0.3

        scores = bboxes[:, -1]
        inds = scores > score_thr
        bboxes = bboxes[inds, :]
        labels = labels[inds]

        # for bbox, label in zip(bboxes, labels):
        #     loc = list(map(int, bbox[:4]))
        #     score = bbox[-1]
        #     print(loc, score, self.CLASSES[label])
        result = list(
            zip([self.model.CLASSES[label] for label in labels],
                bboxes[:,-1],
                [list(map(int, bbox[:4])) for bbox in bboxes]
                )
        )
        return result

def load_model():
    fashion_detect = FashionDetect()
    return fashion_detect

if __name__ == "__main__":
    fashion_detect = FashionDetect()
    image = cv2.imread("../../images/test/img_00000031.jpg")
    image = cv2.cvtColor(image, cv2.COLOR_BGR2RGB)
    result = fashion_detect.detect(image)
    print(result)
