////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"crypto/rand"
	"gitlab.com/elixxir/comms/mixmessages"
	"gitlab.com/elixxir/comms/network"
	ds "gitlab.com/elixxir/comms/network/dataStructures"
	"gitlab.com/elixxir/comms/testutils"
	"gitlab.com/elixxir/primitives/states"
	"gitlab.com/xx_network/crypto/signature/ec"
	"gitlab.com/xx_network/primitives/ndf"
	"testing"
)

func TestFilteredUpdates_RoundUpdate(t *testing.T) {
	validUpdateId := uint64(4)
	validMsg := &mixmessages.RoundInfo{
		ID:         2,
		UpdateID:   validUpdateId,
		State:      uint32(states.COMPLETED),
		BatchSize:  8,
		Timestamps: []uint64{0, 1, 2, 3, 4, 5},
	}

	ecPrivKey, err := ec.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	pubKey := ecPrivKey.GetPublic()

	fullNdf, err := ds.NewNdf(&ndf.NetworkDefinition{
		Registration: ndf.Registration{EllipticPubKey: pubKey.MarshalText()},
	})
	if err != nil {
		t.Fatalf("Failed to generate a mock ndf: %v", err)
	}

	netInf, err := network.NewInstance(gatewayInstance.Comms.ProtoComms, fullNdf.Get(), fullNdf.Get(), nil, network.Lazy, true)
	testFilter, err := NewFilteredUpdates(netInf)
	if err != nil {
		t.Fatalf("Failed to create filtered update: %v", err)
	}
	err = testutils.SignRoundInfoEddsa(validMsg, ecPrivKey, t)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	err = testFilter.RoundUpdate(validMsg)
	// Fixme
	/*	if err == nil {
		t.Error("Should have failed to veNewSecuredNdfrify")
	}*/

	t.Logf("err update: %v", err)

	retrieved, err := testFilter.GetRoundUpdate(int(validMsg.UpdateID))
	if err != nil || retrieved == nil {
		t.Logf("retrieved: %v", retrieved)
		t.Logf("err: %v", err)
		t.Errorf("Should have stored msg with state %s", states.Round(validMsg.State))
	}

	invalidUpdateId := uint64(5)
	invalidMsg := &mixmessages.RoundInfo{
		ID:        2,
		UpdateID:  invalidUpdateId,
		State:     uint32(states.PRECOMPUTING),
		BatchSize: 8,
	}

	err = testFilter.RoundUpdate(invalidMsg)
	if err != nil {
		t.Errorf("Failed to update round: %v", err)
	}

	retrieved, err = testFilter.GetRoundUpdate(int(invalidUpdateId))
	if err == nil || retrieved != nil {
		t.Errorf("Should not have inserted round with state %s",
			states.Round(invalidMsg.State))
	}

}

func TestFilteredUpdates_RoundUpdates(t *testing.T) {
	validUpdateId := uint64(4)
	validMsg := &mixmessages.RoundInfo{
		ID:         2,
		UpdateID:   validUpdateId,
		State:      uint32(states.COMPLETED),
		BatchSize:  8,
		Timestamps: []uint64{0, 1, 2, 3, 4, 5},
	}

	ellipticKey, err := ec.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test ellitpic key: %v", err)
	}

	fullNdf, err := ds.NewNdf(&ndf.NetworkDefinition{
		Registration: ndf.Registration{
			EllipticPubKey: ellipticKey.GetPublic().MarshalText(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to generate a mock ndf: %v", err)
	}

	netInf, err := network.NewInstance(gatewayInstance.Comms.ProtoComms, fullNdf.Get(), fullNdf.Get(), nil, network.Lazy, true)
	if err != nil {
		t.Fatalf("Failed to generate instance: %v", err)
	}

	testFilter, err := NewFilteredUpdates(netInf)
	if err != nil {
		t.Fatalf("Failed to create filtered update: %v", err)
	}
	invalidUpdateId := uint64(5)
	invalidMsg := &mixmessages.RoundInfo{
		ID:         2,
		UpdateID:   invalidUpdateId,
		State:      uint32(states.PRECOMPUTING),
		BatchSize:  8,
		Timestamps: []uint64{0, 1},
	}

	err = testutils.SignRoundInfoEddsa(validMsg, ellipticKey, t)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	err = testutils.SignRoundInfoEddsa(invalidMsg, ellipticKey, t)
	if err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	roundUpdates := []*mixmessages.RoundInfo{validMsg, invalidMsg}

	err = testFilter.RoundUpdates(roundUpdates)
	if err != nil {
		t.Error("Should have failed to get perm host")
	}

	retrieved, err := testFilter.GetRoundUpdate(int(validUpdateId))
	if err != nil || retrieved == nil {
		t.Errorf("Should have stored msg with state %s", states.Round(validMsg.State))
	}

	retrieved, err = testFilter.GetRoundUpdate(int(invalidUpdateId))
	if err == nil || retrieved != nil {
		t.Errorf("Should not have inserted round with state %s",
			states.Round(invalidMsg.State))
	}

}

func TestFilteredUpdates_GetRoundUpdate(t *testing.T) {
	ellipticKey, err := ec.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test ellitpic key: %v", err)
	}

	fullNdf, err := ds.NewNdf(&ndf.NetworkDefinition{
		Registration: ndf.Registration{
			EllipticPubKey: ellipticKey.GetPublic().MarshalText(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to generate a mock ndf: %v", err)
	}

	netInf, err := network.NewInstance(gatewayInstance.Comms.ProtoComms, fullNdf.Get(), fullNdf.Get(), nil, network.Lazy, true)
	if err != nil {
		t.Fatalf("Failed to generate instance: %v", err)
	}

	testFilter, err := NewFilteredUpdates(netInf)
	if err != nil {
		t.Fatalf("Failed to create filtered update: %v", err)
	}
	ri := &mixmessages.RoundInfo{
		ID:         uint64(1),
		UpdateID:   uint64(1),
		State:      uint32(states.QUEUED),
		Timestamps: []uint64{0, 1, 2, 3},
	}
	testutils.SignRoundInfoEddsa(ri, ellipticKey, t)
	rnd := ds.NewRound(ri, nil, ellipticKey.GetPublic())

	_ = testFilter.updates.AddRound(rnd)
	r, err := testFilter.GetRoundUpdate(1)
	if err != nil || r == nil {
		t.Errorf("Failed to retrieve round update: %+v", err)
	}
}

func TestFilteredUpdates_GetRoundUpdates(t *testing.T) {
	ellipticKey, err := ec.NewKeyPair(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate test ellitpic key: %v", err)
	}

	fullNdf, err := ds.NewNdf(&ndf.NetworkDefinition{
		Registration: ndf.Registration{
			EllipticPubKey: ellipticKey.GetPublic().MarshalText(),
		},
	})
	if err != nil {
		t.Fatalf("Failed to generate a mock ndf: %v", err)
	}

	netInf, err := network.NewInstance(gatewayInstance.Comms.ProtoComms, fullNdf.Get(), fullNdf.Get(), nil, network.Lazy, true)
	if err != nil {
		t.Fatalf("Failed to generate instance: %v", err)
	}

	testFilter, err := NewFilteredUpdates(netInf)
	if err != nil {
		t.Fatalf("Failed to create filtered update: %v", err)
	}
	roundInfoOne := &mixmessages.RoundInfo{
		ID:         uint64(1),
		UpdateID:   uint64(2),
		State:      uint32(states.QUEUED),
		Timestamps: []uint64{0, 1, 2, 3},
	}
	if err = testutils.SignRoundInfoEddsa(roundInfoOne, ellipticKey, t); err != nil {
		t.Fatalf("Failed to sign round info: %v", err)
	}
	roundInfoTwo := &mixmessages.RoundInfo{
		ID:         uint64(2),
		UpdateID:   uint64(3),
		State:      uint32(states.QUEUED),
		Timestamps: []uint64{0, 1, 2, 3},
	}
	if err = testutils.SignRoundInfoEddsa(roundInfoTwo, ellipticKey, t); err != nil {
		t.Fatalf("Failed to sign round info: %v", err)
	}
	roundOne := ds.NewRound(roundInfoOne, nil, ellipticKey.GetPublic())
	roundTwo := ds.NewRound(roundInfoTwo, nil, ellipticKey.GetPublic())

	_ = testFilter.updates.AddRound(roundOne)
	_ = testFilter.updates.AddRound(roundTwo)
	r := testFilter.GetRoundUpdates(1)
	if len(r) == 0 {
		t.Errorf("Failed to retrieve round updates")
	}

	r = testFilter.GetRoundUpdates(2)
	if len(r) == 0 {
		t.Errorf("Failed to retrieve round updates")
	}

	r = testFilter.GetRoundUpdates(23)
	if len(r) != 0 {
		t.Errorf("Retrieved a round that was never inserted: %v", r)
	}

}
