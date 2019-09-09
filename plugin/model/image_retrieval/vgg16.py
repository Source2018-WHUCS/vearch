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

import cv2
import numpy as np
from PIL import Image
import torch
from torchvision import transforms
import torchvision.models as models

import torch.nn.functional as F

class BaseModel(object):

    def __init__(self):
        self.image_size = 224
        self.dimision = 512
        self.transform = transforms.Compose([
                        transforms.Resize((224, 224), interpolation=3),
                        transforms.ToTensor(),
                        transforms.Normalize(mean=[0.485, 0.456, 0.406], std=[0.229, 0.224, 0.225])
                    ])

    def load_model(self):
        self.device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
        self.model = models.vgg16(pretrained=True, init_weights=False).to(self.device)
        self.model = self.model.eval()
        # self.model.cuda()

    def preprocess_input(self, image):
        image = Image.fromarray(image)
        image = self.transform(image)
        # print(type(image), image.shape)
        # image = cv2.resize(image, (self.image_size, self.image_size))
        # image = image/255.0
        # image = np.subtract(image, 0.5)
        # image = np.multiply(image, 2.0)
        # image = image[np.newaxis,:]
        return image

    def forward(self, x):
        x = torch.stack(x)
        x = x.to(self.device)
        x = self.model.features(x)
        x = self.model.avgpool(x)
        x = F.avg_pool2d(x, kernel_size=x.size()[2:])
        x = torch.squeeze(x,-1)
        x = torch.squeeze(x,-1)
        return self.torch2list(x)

    def torch2list(self, torch_data):
        return torch_data.cpu().detach().numpy().tolist()

def load_model():
    return BaseModel()

def test():
    from PIL import Image
    image1 = cv2.imread("../../images/test/93396423.jpg")
    image2 = cv2.imread("../../images/test/93983706.jpg")
    image2 = cv2.imread("../../images/test/92433654.jpg")
    image2 = cv2.imread("../../images/test/83866084.jpg")
    # image1 = Image.open("../../images/test/93396423.jpg")
    # image2 = Image.open("../../images/test/93983706.jpg")
    # image2 = Image.open("../../images/test/92433654.jpg")
    model = load_model()
    model.load_model()
    tensor1 = model.preprocess_input(image1)
    tensor2 = model.preprocess_input(image2)
    data = [tensor1,tensor2]
    # data = torch.stack([tensor1,tensor2])
    # print(data.shape)
    array1,array2 = model.torch2list(model.forward(data))

    # array1 = model.torch2list(model.forward(model.preprocess_input(image1)))[0]
    # array2 = model.torch2list(model.forward(model.preprocess_input(image2)))[0]
    # array1 = model.torch2list(model.forward(torch.from_numpy(model.preprocess_input(image1)).permute(0,3,1,2).float()))[0]
    # array2 = model.torch2list(model.forward(torch.from_numpy(model.preprocess_input(image2)).permute(0,3,1,2).float()))[0]
    print(np.array(array1).shape, np.array(array2).shape)
    print(np.dot(array1, array2)/(np.linalg.norm(array1) * np.linalg.norm(array2)))


if __name__ == "__main__":
    test()
