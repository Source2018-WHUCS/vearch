## Introduction

In this folder, we add some pre-training models in keras, eg: VGG、xception、resnet、nasnet、densenet and so on. Before you use them, you should download their weights. Here are the download addresses for these models:

| model name             | link                                                         |
| :--------------------- | ------------------------------------------------------------ |
| VGG16                  | https://github.com/fchollet/deep-learning-models/releases/download/v0.1/vgg16_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| VGG19                  | https://github.com/fchollet/deep-learning-models/releases/download/v0.1/vgg19_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| Xception               | https://github.com/fchollet/deep-learning-models/releases/download/v0.4/xception_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| inception_v3           | https://github.com/fchollet/deep-learning-models/releases/download/v0.5/inception_v3_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| resnet50.py            | https://github.com/fchollet/deep-learning-models/releases/download/v0.2/resnet50_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| inception_resnet_v2.py | https://github.com/fchollet/deep-learning-models/releases/download/v0.7/inception_resnet_v2_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| mobilenet              | https://github.com/fchollet/deep-learning-models/releases/download/v0.6/mobilenet_v2_weights_tf_dim_ordering_tf_kernels_1.0_224_no_top.h5 |
| densenet121            | https://github.com/keras-team/keras-applications/releases/download/densenet/densenet121_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| densenet169            | https://github.com/keras-team/keras-applications/releases/download/densenet/densenet169_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| densenet201            | https://github.com/keras-team/keras-applications/releases/download/densenet/densenet201_weights_tf_dim_ordering_tf_kernels_notop.h5 |
| nasnetmobile           | https://github.com/titu1994/Keras-NASNet/releases/download/v1.2/NASNet-mobile-no-top.h5 |
| nasnetlarge            | https://github.com/titu1994/Keras-NASNet/releases/download/v1.2/NASNet-large-no-top.h5 |

Download what you want, and put them in data folder,then you can use them. You can also add model by inheriting base_model.BaseModel and overloading its function.
