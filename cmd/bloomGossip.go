///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains gossip methods specific to bloom filter gossiping

package cmd

import (
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	ring "gitlab.com/elixxir/bloomfilter"
	pb "gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/xx_network/comms/connect"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/primitives/id"
	"time"
)

// Initialize fields required for the gossip protocol specialized to bloom filters
func (gw *Instance) InitBloomGossip() {

	// Register gossip protocol for bloom filters
	gw.Comms.Manager.NewGossip(BloomFilterGossip, gossip.DefaultProtocolFlags(),
		gw.gossipBloomFilterReceive, gw.gossipVerify, nil)
}

// GossipBloom builds a gossip message containing all of the recipient IDs
// within the bloom filter and gossips it to all peers
func (gw *Instance) GossipBloom(recipientIDs []*id.ID, roundId id.Round) error {
	var err error

	// Build the message
	gossipMsg := &gossip.GossipMsg{
		Tag:    BloomFilterGossip,
		Origin: gw.Comms.Id.Marshal(),
	}

	// Add the GossipMsg payload
	gossipMsg.Payload, err = buildGossipPayloadBloom(recipientIDs, roundId)
	if err != nil {
		return errors.Errorf("Unable to build gossip payload: %+v", err)
	}

	// Add the GossipMsg signature
	gossipMsg.Signature, err = buildGossipSignature(gossipMsg, gw.Comms.GetPrivateKey())
	if err != nil {
		return errors.Errorf("Unable to build gossip signature: %+v", err)
	}

	// Gossip the message
	gossipProtocol, ok := gw.Comms.Manager.Get(BloomFilterGossip)
	if !ok {
		return errors.Errorf("Unable to get gossip protocol.")
	}
	_, errs := gossipProtocol.Gossip(gossipMsg)

	// Return any errors up the stack
	if len(errs) != 0 {
		return errors.Errorf("Could not send to peers: %v", errs)
	}

	return nil
}

// Verify bloom is some additional logic to determine if the
// gateway gossiping a message is responsible for the round
// it has gossiped about.
func verifyBloom(msg *gossip.GossipMsg, origin *id.ID, instance *network.Instance) error {
	// Parse the payload message
	payloadMsg := &pb.Recipients{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected format: %s", err)
	}

	// Check if we recognize the round
	ri, err := instance.GetRound(id.Round(payloadMsg.RoundID))
	if err != nil {
		return errors.Errorf("Did not recognize round sent out by gossip message: %s", err)
	}

	// Parse the round topology
	idList, err := id.NewIDListFromBytes(ri.Topology)
	if err != nil {
		return errors.Errorf("Could not read topology from gossip message: %s", err)
	}
	topology := connect.NewCircuit(idList)

	// Check if the sender is in the round
	//  we have trackerd
	senderIdCopy := origin.DeepCopy()
	senderIdCopy.SetType(id.Node)
	if topology.GetNodeLocation(senderIdCopy) < 0 {
		return errors.New("Origin gateway is not in round it's gossiping about")
	}

	return nil

}

// Receive function for Gossip messages regarding bloom filters
func (gw *Instance) gossipBloomFilterReceive(msg *gossip.GossipMsg) error {
	gw.bloomFilterGossip.Lock()
	// Unmarshal the Recipients data
	payloadMsg := &pb.Recipients{}
	err := proto.Unmarshal(msg.Payload, payloadMsg)
	if err != nil {
		return errors.Errorf("Could not unmarshal message into expected format: %s", err)
	}

	for _, recipient := range payloadMsg.RecipientIds {
		go gw.updateBloomFilter(recipient, payloadMsg.RoundID)

	}

	gw.bloomFilterGossip.Unlock()
	return nil
}

// Update function which updates a recipient's bloom filter.
//  If the client is recognized, we update the user directly
//  If the client is not recognized, we update the ephemeral bloom filter
func (gw *Instance) updateBloomFilter(recipient []byte, roundId uint64) {
	// Marshal the id
	recipientId, err := id.Unmarshal(recipient)
	if err != nil {
		jww.WARN.Printf("Received invalid id through bloom filter gossip: %v", recipient)
		return
	}

	// Attempt to look up the client
	client, err := gw.database.GetClient(recipientId)
	if err != nil || client == nil {
		// If it's an unrecognized client, store to ephemeral
		gw.updateBloomFilter(recipient, roundId)
	} else {

		// Otherwise store as a user
		gw.updateUserBloom(recipientId, client, roundId)
	}

}

// Helper function which updates the clients bloom filter
func (gw *Instance) updateUserBloom(recipientId *id.ID, client *storage.Client, roundId uint64) {
	for _, filter := range client.Filters {
		bloomFilter, err := ring.InitByParameters(bloomFilterSize, bloomFilterHashes)
		if err != nil {
			jww.WARN.Printf("Could not create bloom filter...")
			continue
		}
		err = bloomFilter.UnmarshalBinary(filter.Filter)
		if err != nil {
			jww.WARN.Printf("Could not read filter for ID client: %s", err)
			continue
		}

		// fixme: better way to do this? look into internals of bloom filter
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, roundId)

		bloomFilter.Add(b)

		marshaledFilter, err := bloomFilter.MarshalBinary()
		if err != nil {
			continue
		}

		// fixme: is this right? I don't see an upsert. Would it make more sense to
		//  call insert client with a list of updated filters?
		// fixme: Likely to change due to DB restructure
		err = gw.database.InsertBloomFilter(&storage.BloomFilter{
			ClientId:    recipientId.Bytes(),
			Count:       filter.Count,
			Filter:      marshaledFilter,
			DateCreated: filter.DateCreated,
		})

		if err != nil {
			continue
		}
	}

}

// Helper function which updates the ephemeral bloom filter
func (gw *Instance) updateEphemeralBloom(recipient *id.ID, roundId uint64) {
	// Get the recipient ID if it exists
	filters, err := gw.database.GetEphemeralBloomFilters(recipient)
	if err != nil {
		// todo: create an ephemeral ID here?
		return
	}

	for _, filter := range filters {
		bloomFilter, err := ring.InitByParameters(bloomFilterSize, bloomFilterHashes)
		if err != nil {
			jww.WARN.Printf("Could not create bloom filter...")
			continue
		}
		err = bloomFilter.UnmarshalBinary(filter.Filter)
		if err != nil {
			jww.WARN.Printf("Could not read filter for ID client: %s", err)
			continue
		}

		// fixme: better way to do this? look into internals of bloom filter
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, roundId)

		bloomFilter.Add(b)

		marshaledBloom, err := bloomFilter.MarshalBinary()
		if err != nil {
			continue
		}

		// fixme: Likely to change due to DB restructure
		err = gw.database.InsertEphemeralBloomFilter(&storage.EphemeralBloomFilter{
			RecipientId:  recipient.Bytes(),
			Filter:       marshaledBloom,
			DateModified: time.Now(),
		})

		if err != nil {
			continue
		}
	}
}

// Helper function used to convert recipientIds into a GossipMsg payload
func buildGossipPayloadBloom(recipientIDs []*id.ID, roundId id.Round) ([]byte, error) {
	// Flatten out the new recipients, removing duplicates
	recipientMap := make(map[*id.ID]struct{})
	for _, recipient := range recipientIDs {
		recipientMap[recipient] = struct{}{}
	}

	// Iterate over the map, placing keys back in a list
	// without any duplicates
	i := 0
	recipients := make([][]byte, len(recipientIDs))
	for key := range recipientMap {
		recipients[i] = key.Bytes()
		i++
	}

	// Build the message payload and return
	payloadMsg := &pb.Recipients{
		RecipientIds: recipients,
		RoundID:      uint64(roundId),
	}
	return proto.Marshal(payloadMsg)

}
