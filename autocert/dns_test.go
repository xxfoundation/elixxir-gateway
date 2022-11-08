////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package autocert

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestGenerateCertTestGenerate(t *testing.T) {
	// Regular Gen
	rng1 := &dummyRNG{}
	key, err := GenerateCertKey(rng1)
	if err != nil {
		t.Errorf("%+v", err)
	}

	if key.Size() != 4096/8 {
		t.Errorf("bad key size, expected 4096, got %d", key.Size())
	}

	// Short read error
	// NOTE: The RSA generate function will happily loop forever
	// if the RNG just does an implied short read, it has to return an
	// error!
	rng2 := &shortRNG{}
	key, err = GenerateCertKey(rng2)
	if key != nil || err == nil {
		t.Errorf("expected failure, got success")
	}
}

type dummyRNG struct{}

func (z *dummyRNG) Read(b []byte) (int, error) {
	return rand.Read(b)
}

type shortRNG struct{}

func (z *shortRNG) Read(b []byte) (int, error) {
	k, _ := rand.Read(b[0 : len(b)/2])
	return k, fmt.Errorf("short read")
}
