package mpc

import (
	"github.com/bnb-chain/tss-lib/v2/eddsa/keygen"
	"github.com/bnb-chain/tss-lib/v2/tss"
	"github.com/cryptoniumX/mpcium/pkg/common/errors"
)

type GetRoundFunc func(msg []byte, partyID *tss.PartyID, isBroadcast bool) (RoundInfo, error)

type RoundInfo struct {
	Index         int
	RoundMsg      string
	MsgIdentifier string
}

const (
	EDDSA_KEYGEN1         = "KGRound1Message"
	EDDSA_KEYGEN2aUnicast = "KGRound2Message1"
	EDDSA_KEYGEN2b        = "KGRound2Message2"
	// KEYGEN3              = "KGRound3Message"
	// KEYSIGN1aUnicast     = "SignRound1Message1"
	// KEYSIGN1b            = "SignRound1Message2"
	// KEYSIGN2Unicast      = "SignRound2Message"
	// KEYSIGN3             = "SignRound3Message"
	// KEYSIGN4             = "SignRound4Message"
	// KEYSIGN5             = "SignRound5Message"
	// KEYSIGN6             = "SignRound6Message"
	// KEYSIGN7             = "SignRound7Message"
	// KEYSIGN8             = "SignRound8Message"
	// KEYSIGN9             = "SignRound9Message"
	EDDSA_TSSKEYGENROUNDS = 3
	// TSSKEYSIGNROUNDS     = 10
)

func GetEddsaMsgRound(msg []byte, partyID *tss.PartyID, isBroadcast bool) (RoundInfo, error) {
	parsedMsg, err := tss.ParseWireMessage(msg, partyID, isBroadcast)
	if err != nil {
		return RoundInfo{}, err
	}
	switch parsedMsg.Content().(type) {
	case *keygen.KGRound1Message:
		return RoundInfo{
			Index:    0,
			RoundMsg: EDDSA_KEYGEN1,
		}, nil

	case *keygen.KGRound2Message1:
		return RoundInfo{
			Index:    1,
			RoundMsg: EDDSA_KEYGEN2aUnicast,
		}, nil

	case *keygen.KGRound2Message2:
		return RoundInfo{
			Index:    2,
			RoundMsg: EDDSA_KEYGEN2b,
		}, nil

	default:
		return RoundInfo{}, errors.New("unknown round")
	}
}
