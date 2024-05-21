// This file contains the partial schema of the Vearch REST API.
// i.e. Only fields that are used by the application are specified.
// For a comprehensive reference of the Vearch REST API

package vearch

type upsertBatch struct {
	IDs      []string                 `json:"ids"`
	Payloads []map[string]interface{} `json:"payloads"`
	Vectors  [][]float32              `json:"vectors"`
}

type upsertBody struct {
	Batch upsertBatch `json:"batch"`
}

type result struct {
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

type searchResponse struct {
	Result []result `json:"result"`
}

type searchBody struct {
	Vector         []float32 `json:"vector"`
	Filter         any       `json:"filter"`
	Limit          int       `json:"limit"`
	ScoreThreshold float32   `json:"score_threshold"`
	WithVector     bool      `json:"with_vector"`
	WithPayload    bool      `json:"with_payload"`
}