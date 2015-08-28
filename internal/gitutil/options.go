// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gitutil

type CheckoutOpt interface {
	checkoutOpt()
}
type CommitOpt interface {
	commitOpt()
}
type DeleteBranchOpt interface {
	deleteBranchOpt()
}
type MergeOpt interface {
	mergeOpt()
}
type PushOpt interface {
	pushOpt()
}

type ResetOpt interface {
	resetOpt()
}

type ForceOpt bool

func (ForceOpt) checkoutOpt()     {}
func (ForceOpt) deleteBranchOpt() {}

type MessageOpt string

func (MessageOpt) commitOpt() {}

type ModeOpt string

func (ModeOpt) resetOpt() {}

type ResetOnFailureOpt bool

func (ResetOnFailureOpt) mergeOpt() {}

type SquashOpt bool

func (SquashOpt) mergeOpt() {}

type StrategyOpt string

func (StrategyOpt) mergeOpt() {}

type VerifyOpt bool

func (VerifyOpt) pushOpt() {}
