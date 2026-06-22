// Copyright 2019 The Vearch Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package balancer

import (
	"math"

	"github.com/vearch/vearch/v3/internal/entity"
)

/*
ScoreSpaceOnPS computes a space's "load share" score on a PS.
Formula: spaceScore + sigGroupScore * crossSpaceFactor.
*/
func ScoreSpaceOnPS(s *entity.Space, item *PSItem, snap *Snapshot, cfg *Config) float64 {
	if item.Stat == nil {
		return 0
	}

	// 1. Data volume of this space on this PS.
	var spaceScore float64
	if load, ok := item.Stat.SpaceStats[s.Id]; ok {
		spaceScore = float64(load.DataBytes)
	}

	// 2. Cross-space contribution from spaces sharing the same signature.
	sig := s.Signature()
	var sigGroupScore float64
	for spaceID, load := range item.Stat.SpaceStats {
		if spaceID == s.Id {
			continue
		}
		if snap.signatureOfSpc[spaceID] == sig {
			sigGroupScore += float64(load.DataBytes)
		}
	}

	return spaceScore + sigGroupScore*cfg.CrossSpaceFactor
}

/*
PartitionCountStdDev is the normalized coefficient of variation of per-node
partition counts within one signature.
*/
func PartitionCountStdDev(sig entity.SpaceSignature, snap *Snapshot, items []*PSItem) float64 {
	counts := make([]float64, 0, len(items))
	for _, item := range items {
		counts = append(counts, float64(item.PartitionCountForSig(sig, snap)))
	}
	return stdDevNormalized(counts)
}

/*
DataBytesStdDev is the normalized coefficient of variation of total data
bytes across nodes.
*/
func DataBytesStdDev(items []*PSItem) float64 {
	bytes := make([]float64, 0, len(items))
	for _, item := range items {
		if item.Stat == nil {
			continue
		}
		bytes = append(bytes, float64(item.Stat.TotalDataBytes))
	}
	return stdDevNormalized(bytes)
}

func stdDevNormalized(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum, sumSq float64
	for _, v := range values {
		sum += v
		sumSq += v * v
	}
	n := float64(len(values))
	mean := sum / n
	if mean == 0 {
		return 0
	}
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	return math.Sqrt(variance) / mean
}
