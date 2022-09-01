////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import (
	"github.com/pkg/errors"
	jww "github.com/spf13/jwalterweatherman"
)

// GetEpochEdge determines the Epoch value of the given timestamp with the
// given period while returning an error. To be used when either of the
// inputs come from the network.
func GetEpochEdge(ts int64, period int64) (uint32, error) {
	if period == 0 {
		return 0, errors.New("GetEpochEdge: Period length is 0, " +
			"cannot divide by zero")
	} else if ts < 0 {
		return 0, errors.Errorf("GetEpochEdge: Cannot calculate "+
			"epoch with a negative timestamp: %d", ts)
	} else if period < 0 {
		return 0, errors.Errorf("GetEpochEdge: Cannot calculate "+
			"epoch with a negative period size: %d", period)
	}
	return uint32(ts / period), nil
}

// GetEpoch determines the Epoch value of the given timestamp
// with the given period. Panics on error. For internal use
func GetEpoch(ts int64, period int64) uint32 {
	epoch, err := GetEpochEdge(ts, period)

	if err != nil {
		jww.FATAL.Panicf("%+v", err)
	}

	return epoch
}

// GetEpochTimestamp determines the timestamp value of the given epoch
func GetEpochTimestamp(epoch uint32, period int64) int64 {
	return period * int64(epoch)
}
