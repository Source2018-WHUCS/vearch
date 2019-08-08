#ifndef IVFPQ_PARAM_HELPER_H_
#define IVFPQ_PARAM_HELPER_H_

#include <string>
#include <sstream>
#include "gamma_api.h"
namespace tig_gamma {
class IVFPQParamHelper {
public:
  IVFPQParamHelper(IVFPQParameters *ivfpq_param);
  void SetDefaultValue();
  bool Validate();
  std::string ToString();

private:
  IVFPQParameters *ivfpq_param_;
};
} // namespace tig_gamma

#endif // IVFPQ_PARAM_HELPER_H_
