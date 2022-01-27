/*
 * Copyright (c) 2021 yedf. All rights reserved.
 * Use of this source code is governed by a BSD-style
 * license that can be found in the LICENSE file.
 */

package dtmcli

import (
	"database/sql"
	"errors"

	"github.com/dtm-labs/dtmcli/dtmimp"
)

// Msg reliable msg type
type Msg struct {
	dtmimp.TransBase
}

// NewMsg create new msg
func NewMsg(server string, gid string) *Msg {
	return &Msg{TransBase: *dtmimp.NewTransBase(gid, "msg", server, "")}
}

// Add add a new step
func (s *Msg) Add(action string, postData interface{}) *Msg {
	s.Steps = append(s.Steps, map[string]string{"action": action})
	s.Payloads = append(s.Payloads, dtmimp.MustMarshalString(postData))
	return s
}

// Prepare prepare the msg, msg will later be submitted
func (s *Msg) Prepare(queryPrepared string) error {
	s.QueryPrepared = dtmimp.OrString(queryPrepared, s.QueryPrepared)
	return dtmimp.TransCallDtm(&s.TransBase, s, "prepare")
}

// Submit submit the msg
func (s *Msg) Submit() error {
	return dtmimp.TransCallDtm(&s.TransBase, s, "submit")
}

// DoAndSubmitDB short method for Do on db type. please see DoAndSubmit
func (s *Msg) DoAndSubmitDB(queryPrepared string, db *sql.DB, busiCall BarrierBusiFunc) error {
	return s.DoAndSubmit(queryPrepared, func(bb *BranchBarrier) error {
		return bb.CallWithDB(db, busiCall)
	})
}

// DoAndSubmit one method for the entire prepare->busi->submit
// the error returned by busiCall will be returned
// if busiCall return ErrFailure, then abort is called directly
// if busiCall return not nil error other than ErrFailure, then DoAndSubmit will call queryPrepared to get the result
func (s *Msg) DoAndSubmit(queryPrepared string, busiCall func(bb *BranchBarrier) error) error {
	bb, err := BarrierFrom(s.TransType, s.Gid, "00", "msg") // a special barrier for msg QueryPrepared
	if err == nil {
		err = s.Prepare(queryPrepared)
	}
	if err == nil {
		errb := busiCall(bb)
		if errb != nil && !errors.Is(errb, ErrFailure) {
			// if busicall return an error other than failure, we will query the result
			_, err = dtmimp.TransRequestBranch(&s.TransBase, "GET", nil, bb.BranchID, bb.Op, queryPrepared)
		}
		if errors.Is(errb, ErrFailure) || errors.Is(err, ErrFailure) {
			_ = dtmimp.TransCallDtm(&s.TransBase, s, "abort")
		} else if err == nil {
			err = s.Submit()
		}
		if errb != nil {
			return errb
		}
	}
	return err
}
