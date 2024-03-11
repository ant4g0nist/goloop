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
	"github.com/icon-project/goloop/common"
	"github.com/icon-project/goloop/common/containerdb"
	"github.com/icon-project/goloop/common/intconv"
	"github.com/icon-project/goloop/icon/iiss/icobject"
	"github.com/icon-project/goloop/icon/iiss/icreward"
	"github.com/icon-project/goloop/icon/iiss/icstage"
	"github.com/icon-project/goloop/icon/iiss/icstate"
)

type iiss4Reward struct {
	Context
	g  icstage.Global
	pi *PRepInfo
	ve *VoteEvents
}

func (r *iiss4Reward) Calculate() error {
	r.Logger().Infof("Start calculation %d", r.g.GetStartHeight())
	r.Logger().Infof("Global Option: %+v", r.g)

	var err error
	if err = processClaim(r); err != nil {
		return err
	}

	if err = r.loadPRepInfo(); err != nil {
		return err
	}

	if err = r.processEvents(); err != nil {
		return err
	}

	if r.g.GetElectedPRepCount() == 0 {
		r.Logger().Info("there is no elected PRep. skip reward calculation")
	} else {
		if err = r.processPrepReward(); err != nil {
			return err
		}

		if err = r.processVoterReward(); err != nil {
			return err
		}
	}

	if err = processBTP(r); err != nil {
		return err
	}

	if err = processCommissionRate(r); err != nil {
		return err
	}

	return nil
}

// loadPRepInfo make new PRepInfo and load data from base.VotedV1
func (r *iiss4Reward) loadPRepInfo() error {
	var err error
	var dsa *icreward.DSA
	base := r.Base()

	if dsa, err = base.GetDSA(); err != nil {
		return err
	}

	pi := NewPRepInfo(r.g.GetBondRequirement(), r.g.GetElectedPRepCount(), r.g.GetOffsetLimit(), r.Logger())

	prefix := icreward.VotedKey.Build()
	for iter := base.Filter(prefix); iter.Has(); iter.Next() {
		o, key, err := iter.Get()
		if err != nil {
			return err
		}
		keySplit, err := containerdb.SplitKeys(key)
		if err != nil {
			return err
		}
		addr, err := common.NewAddress(keySplit[1])
		if err != nil {
			return err
		}
		voted := icreward.ToVoted(o)
		pubKey, err := base.GetPublicKey(addr)
		if err != nil {
			return err
		}
		pi.Add(addr, voted.Status(), voted.Delegated(), voted.Bonded(), voted.CommissionRate(), pubKey.HasAll(dsa.Mask()))
	}
	pi.Sort()
	pi.InitAccumulated()

	r.pi = pi

	return nil
}

func (r *iiss4Reward) processEvents() error {
	ve := NewVoteEvents()
	back := r.Back()
	eventPrefix := icstage.EventKey.Build()
	for iter := back.Filter(eventPrefix); iter.Has(); iter.Next() {
		o, key, err := iter.Get()
		if err != nil {
			return err
		}
		type_ := o.(*icobject.Object).Tag().Type()
		keySplit, err := containerdb.SplitKeys(key)
		if err != nil {
			return err
		}
		keyOffset := int(intconv.BytesToInt64(keySplit[1]))
		switch type_ {
		case icstage.TypeEventEnable:
			obj := icstage.ToEventEnable(o)
			r.Logger().Tracef("get event at %d %+v", int(r.g.GetStartHeight())+keyOffset, obj)
			r.pi.SetStatus(obj.Target(), obj.Status())
		case icstage.TypeEventDelegation, icstage.TypeEventBond:
			obj := icstage.ToEventVote(o)
			r.Logger().Tracef("get event at %d %+v", int(r.g.GetStartHeight())+keyOffset, obj)
			vType := vtDelegate
			if type_ == icstage.TypeEventBond {
				vType = vtBond
			}
			r.pi.ApplyVote(vType, obj.Votes(), keyOffset)
			ve.AddEvent(vType, obj.From(), obj.Votes(), keyOffset)
		}
	}
	r.pi.UpdateTotalAccumulatedPower()
	r.ve = ve

	return r.UpdateVoteInfo()
}

// UpdateVoteInfo writes Voted, Bonding and Delegating to temp
func (r *iiss4Reward) UpdateVoteInfo() error {
	base := r.Base()
	temp := r.Temp()
	if err := r.pi.UpdateVoted(temp); err != nil {
		return err
	}
	if err := r.ve.UpdateVoting(base, temp); err != nil {
		return err
	}
	return nil
}

// processPrepReward calculates commission and wage of PRep and writes to icreward.IScore.
func (r *iiss4Reward) processPrepReward() error {
	global := r.g.GetV3()
	err := r.pi.CalculateReward(
		global.GetRewardFundAmountByKey(icstate.KeyIprep),
		global.GetRewardFundAmountByKey(icstate.KeyIwage),
		global.MinBond(),
	)
	if err != nil {
		return err
	}

	for _, prep := range r.pi.PReps() {
		if err = r.UpdateIScore(prep.Owner(), prep.GetReward(), RTPRep); err != nil {
			return err
		}
	}
	return nil
}

// processVoterReward calculates voter reward of all ICONist who has bond or delegation and writes to icreward.IScore.
func (r *iiss4Reward) processVoterReward() error {
	base := r.Base()

	prefix := icreward.DelegatingKey.Build()
	for iter := base.Filter(prefix); iter.Has(); iter.Next() {
		o, key, err := iter.Get()
		if err != nil {
			return err
		}
		d := icreward.ToDelegating(o)
		if d == nil || d.IsEmpty() {
			continue
		}
		var keySplit [][]byte
		keySplit, err = containerdb.SplitKeys(key)
		if err != nil {
			return err
		}
		var addr *common.Address
		addr, err = common.NewAddress(keySplit[1])
		if err != nil {
			return err
		}
		voter := NewVoter(addr, r.Logger())
		voter.ApplyVoting(d, r.pi.GetTermPeriod())

		b, err := base.GetBonding(addr)
		if err != nil {
			return err
		}
		if b != nil && b.IsEmpty() == false {
			voter.ApplyVoting(b, r.pi.GetTermPeriod())
		}

		events := r.ve.Get(addr)
		if events != nil {
			for _, event := range events {
				voter.ApplyEvent(event, r.pi.OffsetLimit()-event.Offset())
			}
			r.ve.SetCalculated(addr)
		}

		iscore := voter.CalculateReward(r.pi)
		if err = r.UpdateIScore(voter.Owner(), iscore, RTVoter); err != nil {
			return err
		}
	}

	prefix = icreward.BondingKey.Build()
	for iter := base.Filter(prefix); iter.Has(); iter.Next() {
		o, key, err := iter.Get()
		if err != nil {
			return err
		}
		b := icreward.ToBonding(o)
		if b == nil || b.IsEmpty() {
			continue
		}
		var keySplit [][]byte
		keySplit, err = containerdb.SplitKeys(key)
		if err != nil {
			return err
		}
		var addr *common.Address
		addr, err = common.NewAddress(keySplit[1])
		if err != nil {
			return err
		}

		d, err := base.GetDelegating(addr)
		if err != nil {
			return err
		}
		if d != nil && !d.IsEmpty() {
			continue
		}

		voter := NewVoter(addr, r.Logger())
		voter.ApplyVoting(b, r.pi.GetTermPeriod())

		events := r.ve.Get(addr)
		if events != nil {
			for _, event := range events {
				voter.ApplyEvent(event, r.pi.OffsetLimit()-event.Offset())
			}
			r.ve.SetCalculated(addr)
		}

		iscore := voter.CalculateReward(r.pi)
		if err = r.UpdateIScore(voter.Owner(), iscore, RTVoter); err != nil {
			return err
		}
	}

	for key, events := range r.ve.Events() {
		if r.ve.IsCalculated(key) {
			continue
		}
		addr, err := common.NewAddress([]byte(key))
		if err != nil {
			return err
		}
		voter := NewVoter(addr, r.Logger())
		for _, event := range events {
			voter.ApplyEvent(event, r.pi.OffsetLimit()-event.Offset())
		}
		r.ve.SetCalculated(addr)

		iscore := voter.CalculateReward(r.pi)
		if err = r.UpdateIScore(voter.Owner(), iscore, RTVoter); err != nil {
			return err
		}
	}

	return nil
}

func NewIISS4Reward(c Context) (*iiss4Reward, error) {
	global, err := c.Back().GetGlobal()
	if err != nil {
		return nil, err
	}
	return &iiss4Reward{Context: c, g: global}, nil
}
