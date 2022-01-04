///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// Contains functionality related to inter-gateway gossip

package cmd

import (
	"crypto/rand"
	"fmt"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/xx_network/comms/gossip"
	"gitlab.com/xx_network/crypto/signature/rsa"
	"gitlab.com/xx_network/primitives/id"
)

// Tag for the client rate limit gossip protocol
const RateLimitGossip = "clientRateLimit"

// Tag for the bloom filter gossip
const BloomFilterGossip = "bloomFilter"

// Starts a thread for monitoring and handling changes to gossip peers
func (gw *Instance) StartPeersThread() {
	go func() {
		fmt.Println("a")
		rateLimitProtocol, exists := gw.Comms.Manager.Get(RateLimitGossip)
		if !exists {
			jww.ERROR.Printf("Unable to get gossip rateLimitProtocol!")
			return
		}
		fmt.Println("b")
		bloomProtocol, exists := gw.Comms.Manager.Get(BloomFilterGossip)
		if !exists {
			jww.ERROR.Printf("Unable to get gossip BloomFilter!")
			return
		}
		fmt.Println("c")

		//add all previously present gateways
		/*for _, gateway := range gw.NetInf.GetFullNdf().Get().Gateways{
			gwId, err := id.Unmarshal(gateway.ID)
			if err != nil {
				jww.WARN.Printf("Unable to unmarshal gossip peer: %+v", err)
				continue
			}
			jww.INFO.Printf("Added %s to gossip peers list", gwId)
			err = rateLimitProtocol.AddGossipPeer(gwId)
			if err != nil {
				jww.WARN.Printf("Unable to add rate limit gossip peer: %+v", err)
			}
			err = bloomProtocol.AddGossipPeer(gwId)
			if err != nil {
				jww.WARN.Printf("Unable to add bloom gossip peer: %+v", err)
			}
		}*/

		for {
			select {
			// TODO: Add kill case?
			case removeId := <-gw.removeGateway:
				jww.INFO.Printf("Removed %s to gossip peers list", removeId)
				err := rateLimitProtocol.RemoveGossipPeer(removeId)
				if err != nil {
					jww.WARN.Printf("Unable to remove rate limit gossip peer: %+v", err)
				}
				err = bloomProtocol.RemoveGossipPeer(removeId)
				if err != nil {
					jww.WARN.Printf("Unable to remove bloom gossip peer: %+v", err)
				}
			case add := <-gw.addGateway:
				fmt.Println("added")
				gwId, err := id.Unmarshal(add.Gateway.ID)
				if err != nil {
					jww.WARN.Printf("Unable to unmarshal gossip peer: %+v", err)
					continue
				}
				jww.INFO.Printf("Added %s to gossip peers list", gwId)
				err = rateLimitProtocol.AddGossipPeer(gwId)
				if err != nil {
					jww.WARN.Printf("Unable to add rate limit gossip peer: %+v", err)
				}
				err = bloomProtocol.AddGossipPeer(gwId)
				if err != nil {
					jww.WARN.Printf("Unable to add bloom gossip peer: %+v", err)
				}
			}
		}
	}()
}

// Verify function for Gossip messages
func (gw *Instance) gossipVerify(msg *gossip.GossipMsg, _ []byte) error {
	// Locate origin host
	origin, err := id.Unmarshal(msg.Origin)
	if err != nil {
		return errors.Errorf("Unable to unmarshal origin: %+v", err)
	}
	host, exists := gw.Comms.GetHost(origin)
	if !exists {
		return errors.Errorf("Unable to locate origin host: %+v", err)
	}

	// Prepare message hash
	options := rsa.NewDefaultOptions()
	hash := options.Hash.New()
	hash.Write(gossip.Marshal(msg))
	hashed := hash.Sum(nil)

	// Verify signature of message using origin host's public key
	err = rsa.Verify(host.GetPubKey(), options.Hash, hashed, msg.Signature, nil)
	if err != nil {
		return errors.Errorf("Unable to verify signature: %+v", err)
	}

	if msg.Tag == RateLimitGossip {
		return nil
	} else if msg.Tag == BloomFilterGossip {
		return nil
	}

	return errors.Errorf("Unrecognized tag: %s", msg.Tag)

}

// Helper function used to obtain Signature bytes of a given GossipMsg
func buildGossipSignature(gossipMsg *gossip.GossipMsg, privKey *rsa.PrivateKey) ([]byte, error) {
	// Hash the message
	options := rsa.NewDefaultOptions()
	hash := options.Hash.New()
	hash.Write(gossip.Marshal(gossipMsg))
	hashed := hash.Sum(nil)

	// Sign the message
	return rsa.Sign(rand.Reader, privKey,
		options.Hash, hashed, nil)
}
