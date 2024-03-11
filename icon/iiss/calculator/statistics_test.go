/*
 * Copyright 2023 ICON Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package calculator

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

type sValue struct {
	rType RewardType
	value int64
}

func TestStats_increase(t *testing.T) {
	stats := NewStats()

	tests := []sValue{
		{RTBlockProduce, 1000},
		{RTBlockProduce, 5},
		{RTPRep, 321},
		{RTPRep, 44444},
		{RTVoter, 100000000},
		{RTVoter, 123},
	}

	for _, tt := range tests {
		old := stats.GetValue(tt.rType)
		count := stats.GetCount(tt.rType)
		value := big.NewInt(tt.value)
		stats.IncreaseReward(tt.rType, value)
		current := stats.GetValue(tt.rType)
		assert.Equal(t, value, new(big.Int).Sub(current, old))
		assert.Equal(t, count+1, stats.GetCount(tt.rType))
	}
}

func TestStats_Total(t *testing.T) {
	tests := []struct {
		values []sValue
		want   int64
	}{
		{
			[]sValue{
				sValue{RTBlockProduce, 1000},
			},
			1000,
		},
		{
			[]sValue{
				sValue{RTPRep, 2000},
			},
			2000,
		},
		{
			[]sValue{
				sValue{RTVoter, 4000},
			},
			4000,
		},
		{
			[]sValue{
				sValue{RTBlockProduce, 1000},
				sValue{RTPRep, 2000},
			},
			3000,
		},
		{
			[]sValue{
				sValue{RTBlockProduce, 1000},
				sValue{RTVoter, 4000},
			},
			5000,
		},
		{
			[]sValue{
				sValue{RTPRep, 2000},
				sValue{RTVoter, 4000},
				sValue{RTVoter, 1},
				sValue{RTVoter, 2},
			},
			6003,
		},
		{
			[]sValue{
				sValue{RTBlockProduce, 1000},
				sValue{RTPRep, 2000},
				sValue{RTVoter, 4000},
			},
			7000,
		},
	}

	for i, tt := range tests {
		stats := NewStats()
		values := map[RewardType]int64{
			RTBlockProduce: 0,
			RTPRep:         0,
			RTVoter:        0,
		}
		counts := map[RewardType]int64{
			RTBlockProduce: 0,
			RTPRep:         0,
			RTVoter:        0,
		}
		for _, v := range tt.values {
			stats.IncreaseReward(v.rType, big.NewInt(v.value))
			values[v.rType] += v.value
			counts[v.rType]++
		}
		assert.Equal(t, tt.want, stats.Total().Int64(), fmt.Sprintf("Index: %d", i))
		for k, _ := range stats.count {
			assert.Equal(t, counts[k], stats.GetCount(k))
			assert.Equal(t, values[k], stats.GetValue(k).Int64())
		}
	}
}
