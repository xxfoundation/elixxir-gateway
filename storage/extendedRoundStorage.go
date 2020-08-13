///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Bridge for ExtendedRoundStorage between comms and the database
// Allows Gateway to store round information longer than would be stored in the
// ring buffer.

package storage

import (
	"github.com/golang/protobuf/proto"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
	"strings"
)

type ERS struct{}

// Store a new round info object into the map
func (ers *ERS) Store(ri *pb.RoundInfo) error {
	// Marshal the data so it can be stored
	m, err := proto.Marshal(ri)
	if err != nil {
		return err
	}

	// Create our DB Round object to store
	dbr := Round{
		Id:       ri.ID,
		UpdateId: ri.UpdateID,
		InfoBlob: m,
	}

	// Store it
	err = GatewayDB.UpsertRound(&dbr)
	if err != nil {
		return err
	}
	return nil
}

// Get a round info object from the memory map database
func (ers *ERS) Retrieve(id id.Round) (*pb.RoundInfo, error) {
	// Retrieve round from the database
	dbr, err := GatewayDB.GetRound(id)
	// Detect if we have an error, if it is because the round couldn't be found
	// we suppress it. Otherwise, bring it up the path.
	if err != nil {
		if strings.HasPrefix(err.Error(), "Could not find Round with ID ") {
			return nil, nil
		} else {
			return nil, err
		}
	}

	// Convert it to a pb.RoundInfo object
	u := &pb.RoundInfo{}
	err = proto.Unmarshal(dbr.InfoBlob, u)
	if err != nil {
		return nil, err
	}

	// Return it
	return u, nil
}

// Get multiple specific round info objects from the memory map database
func (ers *ERS) RetrieveMany(rounds []id.Round) ([]*pb.RoundInfo, error) {
	var r []*pb.RoundInfo

	// Iterate over all rounds provided and put them in the round array
	retrounds, err := GatewayDB.GetRounds(rounds)
	for _, round := range retrounds {
		// Convert it to a pb.RoundInfo object
		u := &pb.RoundInfo{}
		err = proto.Unmarshal(round.InfoBlob, u)
		if err != nil {
			return nil, err
		}
		r = append(r, u)
	}

	return r, nil
}

// Retrieve a concurrent range of round info objects from the memory map database
func (ers *ERS) RetrieveRange(first, last id.Round) ([]*pb.RoundInfo, error) {
	idrange := uint64(last - first)
	i := uint64(0)

	var r []*pb.RoundInfo

	// Iterate over all IDs in the range, retrieving them and putting them in the
	// round array
	for i < idrange+1 {
		ri, err := ers.Retrieve(id.Round(uint64(first) + i))
		if err != nil {
			return nil, err
		}

		if ri != nil {
			r = append(r, ri)
		}
		i++
	}

	return r, nil
}
