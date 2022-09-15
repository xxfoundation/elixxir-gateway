////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"bytes"
	"encoding/base64"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/knownRounds"
	"gitlab.com/xx_network/primitives/rateLimiting"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Unit test of newKnownRoundsWrapper.
func Test_newKnownRoundsWrapper(t *testing.T) {
	// Create new gateway instance
	params := Params{DevMode: true}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	gw := NewGatewayInstance(params)
	roundCapacity := 255
	expected := &knownRoundsWrapper{
		kr: knownRounds.NewKnownRound(roundCapacity),
	}
	expected.kr.Check(0)
	expected.marshalled = expected.kr.Marshal()
	expected.truncated = expected.marshalled
	expectedData := base64.StdEncoding.EncodeToString(expected.marshalled)

	krw, err := newKnownRoundsWrapper(roundCapacity, gw.storage)
	if err != nil {
		t.Errorf("newKnownRoundsWrapper returned an error: %+v", err)
	}

	if !reflect.DeepEqual(expected, krw) {
		t.Errorf("newKnownRoundsWrapper failed to return the expected "+
			"knownRoundsWrapper.\nexpected: %+v\nreceived: %+v", expected, krw)
	}

	data, err := gw.storage.GetStateValue(storage.KnownRoundsKey)
	if err != nil {
		t.Errorf("Failed to load saved knownRoundsWrapper: %+v", err)
	}

	if data != expectedData {
		t.Errorf("newKnownRoundsWrapper failed to save the expected "+
			"knownRoundsWrapper.\nexpected: %+v\nreceived: %+v", expectedData, data)
	}
}

// Unit test of knownRoundsWrapper.check.
func Test_knownRoundsWrapper_check(t *testing.T) {
	params := Params{DevMode: true}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	gw := NewGatewayInstance(params)
	krw, err := newKnownRoundsWrapper(10, gw.storage)
	if err != nil {
		t.Errorf("Failed to create new knownRoundsWrapper: %+v", err)
	}

	err = krw.check(10, gw.storage)
	if err != nil {
		t.Errorf("check returned an error: %+v", err)
	}

	if !bytes.Equal(krw.marshalled, krw.kr.Marshal()) {
		t.Errorf("check failed to save the expected marshalled bytes."+
			"\nexpected: %+v\nreceived: %+v", krw.kr.Marshal(), krw.marshalled)
	}

	data, err := gw.storage.GetStateValue(storage.KnownRoundsKey)
	if err != nil {
		t.Errorf("Failed to load saved knownRoundsWrapper: %+v", err)
	}

	expectedData := base64.StdEncoding.EncodeToString(krw.marshalled)
	if data != expectedData {
		t.Errorf("check failed to save the expected knownRoundsWrapper."+
			"\nexpected: %+v\nreceived: %+v", expectedData, data)
	}
}

// Unit test of knownRoundsWrapper.forceCheck.
func Test_knownRoundsWrapper_forceCheck(t *testing.T) {
	params := Params{DevMode: true}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	gw := NewGatewayInstance(params)
	krw, err := newKnownRoundsWrapper(10, gw.storage)
	if err != nil {
		t.Errorf("Failed to create new knownRoundsWrapper: %+v", err)
	}

	err = krw.forceCheck(10, gw.storage)
	if err != nil {
		t.Errorf("forceCheck returned an error: %+v", err)
	}

	if !bytes.Equal(krw.marshalled, krw.kr.Marshal()) {
		t.Errorf("forceCheck failed to save the expected marshalled bytes."+
			"\nexpected: %+v\nreceived: %+v", krw.kr.Marshal(), krw.marshalled)
	}

	data, err := gw.storage.GetStateValue(storage.KnownRoundsKey)
	if err != nil {
		t.Errorf("Failed to load saved knownRoundsWrapper: %+v", err)
	}

	expectedData := base64.StdEncoding.EncodeToString(krw.marshalled)
	if data != expectedData {
		t.Errorf("forceCheck failed to save the expected knownRoundsWrapper."+
			"\nexpected: %+v\nreceived: %+v", expectedData, data)
	}
}

// Unit test of knownRoundsWrapper.forceCheck.
func Test_knownRoundsWrapper_getMarshal(t *testing.T) {
	params := Params{DevMode: true}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	gw := NewGatewayInstance(params)
	krw, err := newKnownRoundsWrapper(10, gw.storage)
	if err != nil {
		t.Errorf("Failed to create new knownRoundsWrapper: %+v", err)
	}

	if !bytes.Equal(krw.getMarshal(), krw.kr.Marshal()) {
		t.Errorf("getMarshal did not return the expected bytes."+
			"\nexpected: %+v\nreceived: %+v", krw.kr.Marshal(), krw.getMarshal())
	}
}

// Tests that a knownRoundsWrapper that is saved and loaded matches the
// original.
func Test_knownRoundsWrapper_save_load(t *testing.T) {
	params := Params{DevMode: true}
	params.messageRateLimitParams = &rateLimiting.MapParams{
		Capacity:     10,
		LeakedTokens: 1,
		LeakDuration: 10 * time.Second,
		PollDuration: 10 * time.Second,
		BucketMaxAge: 10 * time.Second,
	}
	gw := NewGatewayInstance(params)
	krw, err := newKnownRoundsWrapper(10, gw.storage)
	if err != nil {
		t.Errorf("Failed to create new knownRoundsWrapper: %+v", err)
	}

	err = krw.save(gw.storage)
	if err != nil {
		t.Errorf("save retuned an error: %+v", err)
	}

	loadedKrw := &knownRoundsWrapper{
		kr: knownRounds.NewKnownRound(10),
	}

	err = loadedKrw.load(gw.storage)
	if err != nil {
		t.Errorf("load retuned an error: %+v", err)
	}

	if !bytes.Equal(loadedKrw.marshalled, krw.marshalled) {
		t.Errorf("Saved and loaded knownRoundsWrapper does not match original."+
			"\nexpected: %+v\nreceived: %+v", krw, loadedKrw)
	}
}

// Tests that knownRoundsWrapper.load returns an error if the state value cannot
// be found
func Test_knownRoundsWrapper_load_GetStateValueError(t *testing.T) {
	store, err := storage.NewStorage("", "", "", "", "", true)
	if err != nil {
		t.Fatalf("failed to create new storage: %+v", err)
	}
	krw := &knownRoundsWrapper{kr: knownRounds.NewKnownRound(10)}
	expectedErr := strings.SplitN(storageGetErr, "%", 2)[0]

	err = krw.load(store)
	if err == nil || !strings.Contains(err.Error(), expectedErr) {
		t.Errorf("load did not return the expected error."+
			"\nexpected: %s\nreceived: %+v", expectedErr, err)
	}
}
