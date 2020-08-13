package storage

import (
	"github.com/golang/protobuf/proto"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
)

type ERS struct{}

// var ers ds.ExternalRoundStorage = storage.ERS{}

// Store a new round info object into the map
func (ers ERS) Store(ri *pb.RoundInfo) error {
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
func (ers ERS) Retrieve(id id.Round) (*pb.RoundInfo, error) {
	// Retrieve round from the database
	dbr, err := GatewayDB.GetRound(&id)
	if err != nil {
		return nil, err
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
func (ers ERS) RetrieveMany(rounds []id.Round) ([]*pb.RoundInfo, error) {
	var r []*pb.RoundInfo

	for _, round := range rounds {
		ri, err := ers.Retrieve(round)
		if err != nil {
			return nil, err
		}

		r = append(r, ri)
	}

	return r, nil
}

// Retrieve a concurrent range of round info objects from the memory map database
func (ers ERS) RetrieveRange(first, last id.Round) ([]*pb.RoundInfo, error) {
	idrange := uint64(last - first)
	i := uint64(0)

	var r []*pb.RoundInfo

	// for some reason <= doesn't work?
	for i < idrange+1 {
		ri, err := ers.Retrieve(id.Round(uint64(first) + i))
		if err != nil {
			return nil, err
		}

		r = append(r, ri)
		i++
	}

	return r, nil
}
