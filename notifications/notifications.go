////////////////////////////////////////////////////////////////////////////////
// Copyright © 2022 xx foundation                                             //
//                                                                            //
// Use of this source code is governed by a license that can be found in the  //
// LICENSE file.                                                              //
////////////////////////////////////////////////////////////////////////////////

// notifications contains the structure and functions for tracking users who should be sent push notifications

package notifications

import "gitlab.com/xx_network/primitives/id"

// UserNotifications stores the list of user ids to be notified
type UserNotifications struct {
	ids []*id.ID
}

// Notify adds a user to the list of users to be notified
// If the user is already in the list, a duplicate record is not added
func (n *UserNotifications) Notify(uid *id.ID) {
	if n.ids == nil {
		n.ids = make([]*id.ID, 0)
	}
	_, found := find(n.ids, uid)
	if found {
		return
	}
	n.ids = append(n.ids, uid)
}

// Notified returns a list of string representations of user ids to be notified
func (n *UserNotifications) Notified() []*id.ID {
	var ret []*id.ID
	for _, uid := range n.ids {
		ret = append(ret, uid)
	}
	n.ids = nil
	return ret
}

// find is a helper method for Notify, used to determine if a given user id is already in its list
func find(slice []*id.ID, val *id.ID) (int, bool) {
	for i, item := range slice {
		if item.Cmp(val) {
			return i, true
		}
	}
	return -1, false
}
