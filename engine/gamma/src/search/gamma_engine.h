#ifndef GAMMA_ENGINE_H_
#define GAMMA_ENGINE_H_

#include "gamma_api.h"
#include "numeric_index.h"
#include "profile.h"
#include "vector_manager.h"

#include <condition_variable>
#include <string>

namespace tig_gamma {

class GammaEngine {
public:
  static GammaEngine *GetInstance(const std::string &index_root_path,
                                  int max_doc_size);

  ~GammaEngine();

  int Setup(int max_doc_size);

  Response *Search(const Request *request);

  int CreateTable(const Table *table);

  int AddDoc(const Doc *doc);

  int AddOrUpdateDoc(const Doc *doc);

  int UpdateDoc(const Doc *doc);

  /**
   * Delete doc
   * @param doc_id
   * @return 0 if successed
   */
  int DelDoc(const std::string &doc_id);

  Doc *GetDocByID(const std::string &id);

  /**
   * blocking to build index
   * @return 0 if exited
   */
  int BuildIndex();

  int GetIndexStatus();

  int Dump();

  int Load();

  int GetDocsNum();

  long GetMemoryBytes();

private:
  GammaEngine(const string &index_root_path);
  std::string index_root_path_;

  char *docids_bitmap_;
  Profile *profile_;
  VectorManager *vec_manager_;
  NI::Indexes *numeric_index_;

  int IndexingNumericFields();
  template <typename T> int _indexingField(const std::string &field);

  int max_docid_;
  int max_doc_size_;

  std::atomic<int> delete_num;

  bool b_running_;
  std::condition_variable running_cv_;

  void PackResults(const GammaResult *gamma_results,
                   Response *response_results);

  enum IndexStatus index_status_;
};

} // namespace tig_gamma
#endif
