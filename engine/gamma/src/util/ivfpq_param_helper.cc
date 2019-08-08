#include "ivfpq_param_helper.h"
#include "log.h"

namespace tig_gamma {
IVFPQParamHelper::IVFPQParamHelper(IVFPQParameters *ivfpq_param) {
  ivfpq_param_ = ivfpq_param;
}
void IVFPQParamHelper::SetDefaultValue() {
  if (ivfpq_param_->metric_type == -1)
    ivfpq_param_->metric_type = InnerProduct;
  if (ivfpq_param_->nprobe == -1)
    ivfpq_param_->nprobe = 10;
  if (ivfpq_param_->ncentroids == -1)
    ivfpq_param_->ncentroids = 256;
  if (ivfpq_param_->nsubvector == -1)
    ivfpq_param_->nsubvector = 32;
  if (ivfpq_param_->nbits_per_idx == -1)
    ivfpq_param_->nbits_per_idx = 8;
}

bool IVFPQParamHelper::Validate() {
  if (ivfpq_param_->metric_type < InnerProduct ||
      ivfpq_param_->metric_type > L2 || ivfpq_param_->nprobe <= 0 ||
      ivfpq_param_->ncentroids <= 0 || ivfpq_param_->nsubvector <= 0 ||
      ivfpq_param_->nbits_per_idx <= 0)
    return false;
  if (ivfpq_param_->nsubvector % 4 != 0) {
    LOG(ERROR) << "only support multiple of 4 now, nsubvector="
               << ivfpq_param_->nsubvector;
    return false;
  }
  if (ivfpq_param_->nbits_per_idx != 8) {
    LOG(ERROR) << "only support 8 now, nbits_per_idx="
               << ivfpq_param_->nbits_per_idx;
    return false;
  }
  if (ivfpq_param_->nprobe > ivfpq_param_->ncentroids) {
    LOG(ERROR) << "nprobe=" << ivfpq_param_->nprobe
               << " > ncentroids=" << ivfpq_param_->ncentroids;
    return false;
  }
  return true;
}

std::string IVFPQParamHelper::ToString() {
  std::stringstream ss;
  ss << "ivfpq parameters: metric_type=" << ivfpq_param_->metric_type
     << ", nprobe=" << ivfpq_param_->nprobe
     << ", ncentroids=" << ivfpq_param_->ncentroids
     << ", nsubvector=" << ivfpq_param_->nsubvector
     << ", nbits_per_idx=" << ivfpq_param_->nbits_per_idx;
  return ss.str();
}
} // namespace tig_gamma
