///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

package cmd

import (
	pb "git.xx.network/elixxir/comms/mixmessages"
	"git.xx.network/elixxir/comms/network"
	ds "git.xx.network/elixxir/comms/network/dataStructures"
	"git.xx.network/elixxir/primitives/states"
	"git.xx.network/xx_network/crypto/signature/ec"
)

type FilteredUpdates struct {
	updates  *ds.Updates
	instance *network.Instance
	ecPubKey *ec.PublicKey
}

func NewFilteredUpdates(instance *network.Instance) (*FilteredUpdates, error) {
	ecPubKey, err := ec.LoadPublicKey(instance.GetEllipticPublicKey())
	if err != nil {
		return nil, err
	}

	return &FilteredUpdates{
		updates:  ds.NewUpdates(),
		instance: instance,
		ecPubKey: ecPubKey,
	}, nil
}

// Get an update ID
func (fu *FilteredUpdates) GetRoundUpdate(updateID int) (*pb.RoundInfo, error) {
	return fu.updates.GetUpdate(updateID)
}

// Get updates from a given round
func (fu *FilteredUpdates) GetRoundUpdates(id int) []*pb.RoundInfo {
	return fu.updates.GetUpdates(id)
}

// get the most recent update id
func (fu *FilteredUpdates) GetLastUpdateID() int {
	return fu.updates.GetLastUpdateID()
}

// Pluralized version of RoundUpdate
func (fu *FilteredUpdates) RoundUpdates(rounds []*pb.RoundInfo) error {
	// Process all rounds passed in
	for _, round := range rounds {
		err := fu.RoundUpdate(round)
		if err != nil {
			return err
		}
	}
	return nil
}

// Add a round to the updates filter
func (fu *FilteredUpdates) RoundUpdate(info *pb.RoundInfo) error {
	switch states.Round(info.State) {
	// Only add to filter states client cares about
	case states.COMPLETED, states.FAILED, states.QUEUED:

		roundCopy := info.DeepCopy()

		// Clear out the rsa signature, keeping the EC signature
		// only for FilteredUpdates
		roundCopy.Signature = nil

		// Create a wrapped round object and store it
		rnd := ds.NewRound(roundCopy, nil, fu.ecPubKey)

		err := fu.updates.AddRound(rnd)
		if err != nil {
			return err
		}
	default:

	}

	return nil
}
