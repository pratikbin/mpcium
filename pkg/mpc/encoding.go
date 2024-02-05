// The Licensed Work is (c) 2022 Sygma
// SPDX-License-Identifier: LGPL-3.0-only
package mpc

import (
	"encoding/json"
	"os"

	"github.com/bnb-chain/tss-lib/v2/tss"
)

type TssMessage struct {
	WalletID    string         `json:"sessionID"`
	MsgBytes    []byte         `json:"msgBytes"`
	IsBroadcast bool           `json:"isBroadcast"`
	From        *tss.PartyID   `json:"from"`
	To          []*tss.PartyID `json:"to"`

	IsToOldCommittee        bool `json:"isToOldCommittee"`
	IsToOldAndNewCommittees bool `json:"isToOldAndNewCommittees"`
}

func MarshalTssMessage(
	walletID string,
	msgBytes []byte,
	isBroadcast bool,
	from *tss.PartyID,
	to []*tss.PartyID,
) ([]byte, error) {
	tssMsg := &TssMessage{
		WalletID:    walletID,
		IsBroadcast: isBroadcast,
		MsgBytes:    msgBytes,
		From:        from,
		To:          to,
	}

	msgBytes, err := json.Marshal(tssMsg)
	if err != nil {
		return []byte{}, err
	}

	return msgBytes, nil
}

func MarshalTssResharingMessage(
	msgBytes []byte,
	isToOldCommittee bool,
	isBroadcast bool,
	isToOldAndNewCommittees bool,
	from *tss.PartyID,
	to []*tss.PartyID,
) ([]byte, error) {
	tssMsg := &TssMessage{
		IsToOldCommittee:        isToOldCommittee,
		IsToOldAndNewCommittees: isToOldAndNewCommittees,
		IsBroadcast:             isBroadcast,
		MsgBytes:                msgBytes,
		From:                    from,
		To:                      to,
	}
	msgBytes, err := json.Marshal(tssMsg)
	if err != nil {
		return []byte{}, err
	}

	return msgBytes, nil
}

func UnmarshalTssMessage(msgBytes []byte) (*TssMessage, error) {
	msg := &TssMessage{}
	err := json.Unmarshal(msgBytes, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

type StartMessage struct {
	Params []byte `json:"params"`
}

func MarshalStartMessage(params []byte) ([]byte, error) {
	startSignMessage := &StartMessage{
		Params: params,
	}

	msgBytes, err := json.Marshal(startSignMessage)
	if err != nil {
		return []byte{}, err
	}

	return msgBytes, nil
}

func UnmarshalStartMessage(msgBytes []byte) (*StartMessage, error) {
	msg := &StartMessage{}
	err := json.Unmarshal(msgBytes, msg)
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func SaveStructToJsonFile(s interface{}, filename string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
