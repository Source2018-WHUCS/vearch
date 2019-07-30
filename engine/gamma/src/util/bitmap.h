#ifndef _SIMPLE_BITMAP_H_
#define _SIMPLE_BITMAP_H_
#include <vector>
static const int SLOT_SIZE = 8 * sizeof(int);

// 位图索引数据对象,32位对齐
namespace bitmap {
// 创建位图指针，count输入是位数, len是分配的内存大小
// is_set 默认不是要置1. true为置1， false全部置0
char *create(int count, int &len, bool is_set = false);
// 根据count实际使用的size
int size(int count);
// 销毁位图
void destroy(char *&bitmap);
// 由调用方判断是否越界, 测试第id位是否置位
bool test(const char *bitmap, int id);
// 第id位置进行置位
void set(char *bitmap, int id);
// 全部置位
void set_all(char *bitmap, int count);
// 第id位置复位
void reset(char *bitmap, int id);
// 全部复位
void reset_all(char *bitmap, int count);
// 用一块内存直接覆盖
int mount(char *bitmap, char *data, int size);
// 用一块内存直接覆盖
int mount(std::vector<int> &bitmap, char *data, int size);
// 打印全部状态
void print(const char *bitmap, int count);
//是否全部复位
bool is_all_reset(char *bitmap, int count);
};

#endif
