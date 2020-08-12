package storage

import (
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/xx_network/primitives/id"
)

type ERS struct{}

// var ers ds.ExternalRoundStorage = storage.ERS{}

// Store a new round info object into the map
func (ers ERS) Store(ri *pb.RoundInfo) error {
	dbr := Round{
		Id:       ri.ID,
		UpdateId: ri.UpdateID,
	}
	err := GatewayDB.UpsertRound(&dbr)
	if err != nil {
		return err
	}
	return nil
}

// Get a round info object from the memory map database
func (ers ERS) Retrieve(id id.Round) (*pb.RoundInfo, error) {
	dbr, err := GatewayDB.GetRound(&id)
	if err != nil {
		return nil, err
	}
	r := pb.RoundInfo{ID: dbr.Id, UpdateID: dbr.UpdateId}
	return &r, nil
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
