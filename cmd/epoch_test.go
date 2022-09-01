////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

package cmd

import "testing"

// Happy path
// Can't test panic paths, obviously
func TestGetEpoch(t *testing.T) {
	ts := int64(300000)
	period := int64(5000)
	expected := uint32(60)
	result := GetEpoch(ts, period)
	if result != expected {
		t.Errorf("Invalid GetEpoch result: Got %d Expected %d", result, expected)
	}
}

// Various happy paths
func TestGetEpochTimestamp(t *testing.T) {
	epoch := uint32(60)
	period := int64(5000)
	expected := int64(300000)
	result := GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}

	period = 0
	expected = 0
	result = GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}

	period = -5000
	expected = -300000
	result = GetEpochTimestamp(epoch, period)
	if result != expected {
		t.Errorf("Invalid GetEpochTimestamp result: Got %d Expected %d", result, expected)
	}
}
