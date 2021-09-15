///////////////////////////////////////////////////////////////////////////////
// Copyright © 2020 xx network SEZC                                          //
//                                                                           //
// Use of this source code is governed by a license that can be found in the //
// LICENSE file                                                              //
///////////////////////////////////////////////////////////////////////////////

// The knownRoundsWrapper contains the gateway's known rounds in a structure
// that saves the marshalled known rounds to memory and the database every time
// a change is made instead of marshalling the known rounds every time the
// marshalled bytes are needed.

package cmd

import (
	"encoding/base64"
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
	"gitlab.com/elixxir/gateway/storage"
	"gitlab.com/elixxir/primitives/knownRounds"
	"gitlab.com/xx_network/primitives/id"
	"sync"
)

// Error messages.
const (
	storageUpsertErr    = "failed to upsert marshalled KnownRounds to storage: %+v"
	storageGetErr       = "failed to get KnownRounds from storage: %+v"
	storageDecodeErr    = "failed to decode KnownRounds from storage: %+v"
	storageUnmarshalErr = "failed to unmarshal KnownRounds from storage: %+v"
)

type knownRoundsWrapper struct {
	kr         *knownRounds.KnownRounds
	marshalled []byte
	l          sync.RWMutex
}

// newKnownRoundsWrapper creates a new knownRoundsWrapper with a new KnownRounds
// initialised to the round capacity and saves marshalled bytes.
func newKnownRoundsWrapper(roundCapacity int, store *storage.Storage) (*knownRoundsWrapper, error) {
	krw := &knownRoundsWrapper{
		kr:         knownRounds.NewKnownRound(roundCapacity),
		marshalled: []byte{},
	}

	// There is no round 0
	krw.kr.Check(0)
	jww.DEBUG.Printf("Initial KnownRound State: %+v", krw.kr)

	// Save marshalled knownRounds to memory and storage
	err := krw.saveUnsafe(store)
	if err != nil {
		return nil, err
	}

	jww.DEBUG.Printf("Initial KnownRound Marshal: %v", krw.marshalled)

	return krw, nil
}

// check force checks the round and saves the KnownRounds.
func (krw *knownRoundsWrapper) check(rid id.Round, store *storage.Storage) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	krw.kr.Check(rid)

	return krw.saveUnsafe(store)
}

// forceCheck force checks the round and saves the KnownRounds.
func (krw *knownRoundsWrapper) forceCheck(rid id.Round, store *storage.Storage) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	krw.kr.ForceCheck(rid)

	return krw.saveUnsafe(store)
}

// getMarshal returns a copy of the marshalled bytes of the KnownRounds.
func (krw *knownRoundsWrapper) getMarshal() []byte {
	krw.l.Lock()
	defer krw.l.Unlock()

	bytes := make([]byte, len(krw.marshalled))
	copy(bytes, krw.marshalled)

	return bytes
}

// save saves the marshalled KnownRounds to memory and storage. This
// function is thread safe.
func (krw *knownRoundsWrapper) save(store *storage.Storage) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	return krw.saveUnsafe(store)
}

// saveUnsafe saves the marshalled KnownRounds but the mutex must be
// locked by the caller.
func (krw *knownRoundsWrapper) saveUnsafe(store *storage.Storage) error {
	// Marshal and save knownRounds
	krw.marshalled = krw.kr.Marshal()

	// Store knownRounds data
	err := store.UpsertState(&storage.State{
		Key:   storage.KnownRoundsKey,
		Value: base64.StdEncoding.EncodeToString(krw.marshalled),
	})
	if err != nil {
		return errors.Errorf(storageUpsertErr, err)
	}

	return nil
}

// load loads the KnownRounds from storage into the knownRoundsWrapper.
func (krw *knownRoundsWrapper) load(store *storage.Storage) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	// Get an existing knownRounds value from storage
	data, err := store.GetStateValue(storage.KnownRoundsKey)
	if err != nil {
		return errors.Errorf(storageGetErr, err)
	}

	dataDecode, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return errors.Errorf(storageDecodeErr, err)
	}

	// Parse the data and store the KnownRounds
	err = krw.kr.Unmarshal(dataDecode)
	if err != nil {
		return errors.Errorf(storageUnmarshalErr, err)
	}

	// Save the marshalled KnownRounds
	krw.marshalled = dataDecode

	return nil
}
