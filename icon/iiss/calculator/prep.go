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
	"bytes"
	"fmt"
	"math/big"
	"sort"

	"github.com/icon-project/goloop/common/log"
	"github.com/icon-project/goloop/icon/icmodule"
	"github.com/icon-project/goloop/icon/iiss/icreward"
	"github.com/icon-project/goloop/icon/iiss/icstage"
	"github.com/icon-project/goloop/icon/iiss/icutils"
	"github.com/icon-project/goloop/module"
)

type PRep struct {
	status         icmodule.EnableStatus
	delegated      *big.Int
	bonded         *big.Int
	commissionRate icmodule.Rate

	owner            module.Address
	power            *big.Int
	pubkey           bool
	rank             int
	accumulatedVoted *big.Int
	accumulatedPower *big.Int
	commission       *big.Int // in IScore
	voterReward      *big.Int // in IScore
	wage             *big.Int // in IScore
}

func (p *PRep) IsElectable() bool {
	return p.pubkey && (p.status == icmodule.ESEnable || p.status == icmodule.ESUnjail)
}

func (p *PRep) IsRewardable(electedPRepCount int) bool {
	return p.status == icmodule.ESEnable && p.rank < electedPRepCount && p.accumulatedPower.Sign() == 1
}

func (p *PRep) Status() icmodule.EnableStatus {
	return p.status
}

func (p *PRep) SetStatus(status icmodule.EnableStatus) {
	p.status = status
}

func (p *PRep) GetVotedValue() *big.Int {
	return new(big.Int).Add(p.delegated, p.bonded)
}

func (p *PRep) Owner() module.Address {
	return p.owner
}

func (p *PRep) UpdatePower(bondRequirement icmodule.Rate) *big.Int {
	p.power = p.calcPower(bondRequirement)
	return p.power
}

func (p *PRep) SetRank(rank int) {
	p.rank = rank
}

func (p *PRep) AccumulatedPower() *big.Int {
	return p.accumulatedPower
}

func (p *PRep) InitAccumulated(termPeriod int64) {
	period := big.NewInt(termPeriod)
	p.accumulatedVoted = new(big.Int).Mul(p.GetVotedValue(), period)
	p.accumulatedPower = new(big.Int).Mul(p.power, period)
}

func (p *PRep) calcPower(bondRequirement icmodule.Rate) *big.Int {
	return icutils.CalcPower(bondRequirement, p.bonded, p.GetVotedValue())
}

// ApplyVote applies vote value to PRep.
// Updates bonded, delegated, power, accumulatedBonded, accumulatedVoted and accumulatedPower
func (p *PRep) ApplyVote(vType VoteType, amount *big.Int, period int, bondRequirement icmodule.Rate) {
	pr := big.NewInt(int64(period))
	if vType == vtBond {
		p.bonded = new(big.Int).Add(p.bonded, amount)
	} else {
		p.delegated = new(big.Int).Add(p.delegated, amount)
	}
	p.accumulatedVoted = new(big.Int).Add(p.accumulatedVoted, new(big.Int).Mul(amount, pr))
	power := p.calcPower(bondRequirement)
	if p.power.Cmp(power) != 0 {
		powerDiff := new(big.Int).Sub(power, p.power)
		p.power = power
		p.accumulatedPower = new(big.Int).Add(p.accumulatedPower, new(big.Int).Mul(powerDiff, pr))
	}
}

func (p *PRep) VoterReward() *big.Int {
	return p.voterReward
}

func (p *PRep) GetReward() *big.Int {
	return new(big.Int).Add(p.commission, p.wage)
}

func (p *PRep) AccumulatedVoted() *big.Int {
	return p.accumulatedVoted
}

func (p *PRep) CalculateReward(totalPRepReward, totalAccumulatedPower, minBond, minWage *big.Int) {
	prepReward := new(big.Int).Mul(totalPRepReward, p.accumulatedPower)
	prepReward.Div(prepReward, totalAccumulatedPower)

	commission := p.commissionRate.MulBigInt(prepReward)
	p.commission = commission
	p.voterReward = new(big.Int).Sub(prepReward, commission)
	if p.bonded.Cmp(minBond) >= 0 {
		p.wage = minWage
	}
}

func (p *PRep) Bigger(p1 *PRep) bool {
	if p.IsElectable() != p1.IsElectable() {
		return p.IsElectable()
	}
	c := p.power.Cmp(p1.power)
	if c != 0 {
		return c == 1
	}
	c = p.delegated.Cmp(p1.delegated)
	if c != 0 {
		return c == 1
	}
	return bytes.Compare(p.owner.Bytes(), p1.owner.Bytes()) > 0
}

func (p *PRep) ToVoted() *icreward.Voted {
	voted := icreward.NewVotedV2()
	if p.Status() == icmodule.ESEnableAtNextTerm {
		voted.SetStatus(icmodule.ESEnable)
	} else {
		voted.SetStatus(p.status)
	}
	voted.SetBonded(p.bonded)
	voted.SetDelegated(p.delegated)
	voted.SetCommissionRate(p.commissionRate)
	return voted
}

func (p *PRep) Equal(p1 *PRep) bool {
	return p.status == p1.status &&
		p.delegated.Cmp(p1.delegated) == 0 &&
		p.bonded.Cmp(p1.bonded) == 0 &&
		p.commissionRate == p1.commissionRate &&
		p.owner.Equal(p1.owner) &&
		p.power.Cmp(p1.power) == 0 &&
		p.pubkey == p1.pubkey &&
		p.rank == p1.rank &&
		p.accumulatedVoted.Cmp(p1.accumulatedVoted) == 0 &&
		p.accumulatedPower.Cmp(p1.accumulatedPower) == 0 &&
		p.commission.Cmp(p1.commission) == 0 &&
		p.voterReward.Cmp(p1.voterReward) == 0 &&
		p.wage.Cmp(p1.wage) == 0
}

func (p *PRep) Clone() *PRep {
	return &PRep{
		owner:            p.owner,
		status:           p.status,
		delegated:        new(big.Int).Set(p.delegated),
		bonded:           new(big.Int).Set(p.bonded),
		commissionRate:   p.commissionRate,
		pubkey:           p.pubkey,
		power:            new(big.Int).Set(p.power),
		accumulatedVoted: new(big.Int).Set(p.accumulatedVoted),
		accumulatedPower: new(big.Int).Set(p.accumulatedPower),
		commission:       new(big.Int).Set(p.commission),
		voterReward:      new(big.Int).Set(p.voterReward),
		wage:             new(big.Int).Set(p.wage),
	}
}
func (p *PRep) Format(f fmt.State, c rune) {
	switch c {
	case 'v':
		if f.Flag('+') {
			fmt.Fprintf(f, "PRep{status=%s delegated=%d bonded=%d commissionRate=%d "+
				"owner=%s power=%d pubkey=%v rank=%d accumulatedVoted=%d accumulatedPower=%d "+
				"commission=%d voterReward=%d wage=%d}",
				p.status, p.delegated, p.bonded, p.commissionRate,
				p.owner, p.power, p.pubkey, p.rank, p.accumulatedVoted, p.accumulatedPower,
				p.commission, p.voterReward, p.wage,
			)
		} else {
			fmt.Fprintf(f, "PRep{%s %d %d %d %s %d %v %d %d %d %d %d %d}",
				p.status, p.delegated, p.bonded, p.commissionRate,
				p.owner, p.power, p.pubkey, p.rank, p.accumulatedVoted, p.accumulatedPower,
				p.commission, p.voterReward, p.wage,
			)
		}
	}
}

func NewPRep(owner module.Address, status icmodule.EnableStatus, delegated, bonded *big.Int,
	commissionRate icmodule.Rate, pubkey bool) *PRep {
	return &PRep{
		owner:            owner,
		status:           status,
		delegated:        delegated,
		bonded:           bonded,
		commissionRate:   commissionRate,
		pubkey:           pubkey,
		power:            new(big.Int),
		accumulatedVoted: new(big.Int),
		accumulatedPower: new(big.Int),
		commission:       new(big.Int),
		voterReward:      new(big.Int),
		wage:             new(big.Int),
	}
}

// PRepInfo stores information for PRep reward calculation.
type PRepInfo struct {
	preps                 map[string]*PRep
	totalAccumulatedPower *big.Int

	electedPRepCount int
	bondRequirement  icmodule.Rate
	offsetLimit      int
	rank             []*PRep
	log              log.Logger
}

func (p *PRepInfo) PReps() map[string]*PRep {
	return p.preps
}

func (p *PRepInfo) GetPRep(key string) *PRep {
	prep, _ := p.preps[key]
	return prep
}

func (p *PRepInfo) ElectedPRepCount() int {
	return p.electedPRepCount
}

func (p *PRepInfo) OffsetLimit() int {
	return p.offsetLimit
}

func (p *PRepInfo) GetTermPeriod() int64 {
	return int64(p.offsetLimit + 1)
}

func (p *PRepInfo) Add(target module.Address, status icmodule.EnableStatus, delegated, bonded *big.Int,
	commissionRate icmodule.Rate, pubkey bool) *PRep {
	prep := NewPRep(target, status, delegated, bonded, commissionRate, pubkey)
	prep.UpdatePower(p.bondRequirement)
	p.preps[icutils.ToKey(target)] = prep
	return prep
}

func (p *PRepInfo) SetStatus(target module.Address, status icmodule.EnableStatus) {
	p.log.Debugf("SetStatus: %s to %d", target, status)
	key := icutils.ToKey(target)
	if prep, ok := p.preps[key]; ok {
		prep.SetStatus(status)
	} else {
		p.Add(target, status, new(big.Int), new(big.Int), 0, false)
	}
}

func (p *PRepInfo) Sort() {
	size := len(p.preps)
	orderedPreps := make([]*PRep, size)
	i := 0
	for _, data := range p.preps {
		orderedPreps[i] = data
		i += 1
	}
	sort.Slice(orderedPreps, func(i, j int) bool {
		return orderedPreps[i].Bigger(orderedPreps[j])
	})
	for idx, prep := range orderedPreps {
		prep.SetRank(idx)
	}
	p.rank = orderedPreps
}

// InitAccumulated update accumulated values of elected PReps
func (p *PRepInfo) InitAccumulated() {
	for i, prep := range p.rank {
		if i >= p.electedPRepCount {
			break
		}
		prep.InitAccumulated(p.GetTermPeriod())
	}
}

func (p *PRepInfo) ApplyVote(vType VoteType, votes icstage.VoteList, offset int) {
	for _, vote := range votes {
		key := icutils.ToKey(vote.To())
		prep, ok := p.preps[key]
		if !ok {
			prep = p.Add(vote.To(), icmodule.ESDisablePermanent, new(big.Int), new(big.Int), 0, false)
		}
		prep.ApplyVote(vType, vote.Amount(), p.offsetLimit-offset, p.bondRequirement)
		p.log.Debugf("ApplyVote %+v: by %d, %d %+v, %d * %d",
			prep, vType, offset, vote, vote.Amount(), p.offsetLimit-offset)
	}
}

// UpdateTotalAccumulatedPower updates totalAccumulatedPower of PRepInfo with accumulatedPower of elected preps
func (p *PRepInfo) UpdateTotalAccumulatedPower() {
	totalAccumulatedPower := new(big.Int)
	for i, prep := range p.rank {
		if i >= p.electedPRepCount {
			break
		}
		accumPower := prep.AccumulatedPower()
		totalAccumulatedPower.Add(totalAccumulatedPower, accumPower)
		p.log.Debugf("[%d] totalAccumulatedPower %d = old + %d by %s", i, totalAccumulatedPower, accumPower, prep.owner)
	}
	p.totalAccumulatedPower = totalAccumulatedPower
}

func fundToPeriodIScore(reward *big.Int, period int64) *big.Int {
	value := new(big.Int).Mul(reward, big.NewInt(period*icmodule.IScoreICXRatio))
	return value.Div(value, big.NewInt(icmodule.MonthBlock))
}

// CalculateReward calculates commission, wage and voter reward of the PRep.
func (p *PRepInfo) CalculateReward(totalReward, totalMinWage, minBond *big.Int) error {
	p.log.Debugf("CalculateReward()")
	if p.electedPRepCount == 0 {
		p.log.Debugf("skip PRepInfo.CalculateReward()")
		return nil
	}
	tReward := fundToPeriodIScore(totalReward, p.GetTermPeriod())
	minWage := fundToPeriodIScore(totalMinWage, p.GetTermPeriod())
	p.log.Debugf("RewardFund: PRep: %d, wage: %d", tReward, minWage)
	minWagePerPRep := new(big.Int).Div(minWage, big.NewInt(int64(p.electedPRepCount)))
	p.log.Debugf("wage to a prep: %d", minWagePerPRep)
	p.log.Debugf("TotalAccumulatedPower: %d", p.totalAccumulatedPower)
	for i, prep := range p.rank {
		if i >= p.electedPRepCount {
			break
		}
		if !prep.IsRewardable(p.electedPRepCount) {
			continue
		}
		prep.CalculateReward(tReward, p.totalAccumulatedPower, minBond, minWagePerPRep)

		p.log.Debugf("rank#%d: %+v", i, prep)
	}
	return nil
}

// UpdateVoted writes updated Voted to database
func (p *PRepInfo) UpdateVoted(writer RewardWriter) error {
	for _, prep := range p.preps {
		err := writer.SetVoted(prep.Owner(), prep.ToVoted())
		if err != nil {
			return err
		}
	}
	return nil
}

func NewPRepInfo(bondRequirement icmodule.Rate, electedPRepCount, offsetLimit int, logger log.Logger) *PRepInfo {
	return &PRepInfo{
		preps:                 make(map[string]*PRep),
		totalAccumulatedPower: icmodule.BigIntZero,
		electedPRepCount:      electedPRepCount,
		bondRequirement:       bondRequirement,
		offsetLimit:           offsetLimit,
		log:                   logger,
	}
}
