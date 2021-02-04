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

// Store a new round info object into the map
func (s *Storage) Store(ri *pb.RoundInfo) error {
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
	err = s.UpsertRound(&dbr)
	if err != nil {
		return err
	}
	return nil
}

// Get a round info object from the memory map database
func (s *Storage) Retrieve(id id.Round) (*pb.RoundInfo, error) {
	// Retrieve round from the database
	dbr, err := s.GetRound(id)
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
func (s *Storage) RetrieveMany(rounds []id.Round) ([]*pb.RoundInfo, error) {
	var r = make([]*pb.RoundInfo, len(rounds))

	// Iterate over all rounds provided and put them in the round array
	for i, rid := range rounds {
		ri, err := s.Retrieve(rid)
		if err != nil {
			return nil, err
		}
		r[i] = ri
	}

	return r, nil
}

// Retrieve a concurrent range of round info objects from the memory map database
func (s *Storage) RetrieveRange(first, last id.Round) ([]*pb.RoundInfo, error) {
	idRange := uint64(last-first) + 1

	var r = make([]*pb.RoundInfo, idRange)

	// Iterate over all IDs in the range, retrieving them and putting them in the
	// round array
	for i := uint64(0); i < idRange; i++ {
		ri, err := s.Retrieve(id.Round(uint64(first) + i))
		if err != nil {
			return nil, err
		}
		r[i] = ri
	}

	return r, nil
}
