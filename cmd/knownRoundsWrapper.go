////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

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
	"time"
)

// Error messages.
const (
	// Determines round differences that triggers a truncate
	knownRoundsTruncateThreshold id.Round = 3000
	storageUpsertErr                      = "failed to upsert marshalled KnownRounds to storage: %+v"
	storageGetErr                         = "failed to get KnownRounds from storage: %+v"
	storageDecodeErr                      = "failed to decode KnownRounds from storage: %+v"
	storageUnmarshalErr                   = "failed to unmarshal KnownRounds from storage: %+v"
)

type knownRoundsWrapper struct {
	kr           *knownRounds.KnownRounds
	marshalled   []byte
	truncated    []byte
	l            sync.RWMutex
	backupChan   chan bool
	backupPeriod time.Duration
}

// newKnownRoundsWrapper creates a new knownRoundsWrapper with a new KnownRounds
// initialised to the round capacity and saves marshalled bytes.
func newKnownRoundsWrapper(roundCapacity int, store *storage.Storage) (*knownRoundsWrapper, error) {
	krw := &knownRoundsWrapper{
		kr:         knownRounds.NewKnownRound(roundCapacity),
		marshalled: []byte{},
		truncated:  []byte{},
		backupChan: make(chan bool, 1),
	}

	krw.backupState(store)

	// There is no round 0
	krw.kr.Check(0)
	jww.TRACE.Printf("Initial KnownRound State: %+v", krw.kr)

	// Save marshalled knownRounds to memory and storage
	err := krw.saveUnsafe()
	if err != nil {
		return nil, err
	}

	jww.DEBUG.Printf("Initial KnownRound Marshal: %v", krw.marshalled)

	krw.backupPeriod = 5 * time.Second

	return krw, nil
}

// check force checks the round and saves the KnownRounds.
func (krw *knownRoundsWrapper) check(rid id.Round) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	krw.kr.Check(rid)

	return krw.saveUnsafe()
}

func (krw *knownRoundsWrapper) truncateMarshal() []byte {
	krw.l.RLock()
	defer krw.l.RUnlock()

	bytes := make([]byte, len(krw.truncated))
	copy(bytes, krw.truncated)

	return bytes
}

func (krw *knownRoundsWrapper) getLastChecked() id.Round {
	krw.l.RLock()
	defer krw.l.RUnlock()

	return krw.kr.GetLastChecked()
}

// forceCheck force checks the round and saves the KnownRounds.
func (krw *knownRoundsWrapper) forceCheck(rid id.Round) error {
	krw.l.Lock()
	defer krw.l.Unlock()

	krw.kr.ForceCheck(rid)

	return krw.saveUnsafe()
}

// getMarshal returns a copy of the marshalled bytes of the KnownRounds.
func (krw *knownRoundsWrapper) getMarshal() []byte {
	krw.l.RLock()
	defer krw.l.RUnlock()

	bytes := make([]byte, len(krw.marshalled))
	copy(bytes, krw.marshalled)

	return bytes
}

// save the marshalled KnownRounds to memory and storage. This
// function is thread safe.
func (krw *knownRoundsWrapper) save() error {
	krw.l.Lock()
	defer krw.l.Unlock()

	return krw.saveUnsafe()
}

// saveUnsafe saves the marshalled KnownRounds but the mutex must be
// locked by the caller.
func (krw *knownRoundsWrapper) saveUnsafe() error {
	// Marshal and save knownRounds
	krw.marshalled = krw.kr.Marshal()
	if krw.kr.GetLastChecked() > knownRoundsTruncateThreshold {
		krw.truncated = krw.kr.Truncate(krw.kr.GetLastChecked() - knownRoundsTruncateThreshold).Marshal()
	} else {
		krw.truncated = krw.marshalled
	}

	select {
	case krw.backupChan <- true:
	default:
	}

	return nil
}

// Store known rounds marshalled in state at most once every 5 seconds
func (krw *knownRoundsWrapper) backupState(store *storage.Storage) {
	go func() {
		for {
			select {
			case <-krw.backupChan:
				// Store knownRounds data
				err := store.UpsertState(&storage.State{
					Key:   storage.KnownRoundsKey,
					Value: base64.StdEncoding.EncodeToString(krw.marshalled),
				})
				if err != nil {
					jww.ERROR.Printf(storageUpsertErr, err)
				}
				time.Sleep(krw.backupPeriod)
			}
		}
	}()
}

// Returns whether the given round calls for a truncated knownRound
func (krw *knownRoundsWrapper) needsTruncated(round id.Round) bool {
	lastChecked := krw.kr.GetLastChecked()
	return round < lastChecked && lastChecked-round > knownRoundsTruncateThreshold
}

// load the KnownRounds from storage into the knownRoundsWrapper.
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
	if krw.kr.GetLastChecked() > knownRoundsTruncateThreshold {
		krw.truncated = krw.kr.Truncate(krw.kr.GetLastChecked() - knownRoundsTruncateThreshold).Marshal()
	} else {
		krw.truncated = dataDecode
	}

	return nil
}
