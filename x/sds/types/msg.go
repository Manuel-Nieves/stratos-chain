package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stratos "github.com/stratosnet/stratos-chain/types"
)

const (
	ConstFileUpload = "FileUploadTx"
	ConstSdsPrepay  = "SdsPrepayTx"
)

type MsgFileUpload struct {
	FileHash string             `json:"file_hash" yaml:"file_hash"` // hash of file
	From     sdk.AccAddress     `json:"from" yaml:"from"`           // wallet addr who will pay this tx
	Reporter stratos.SdsAddress `json:"reporter" yaml:"reporter"`   // p2pAddr of sp node who reports this tx
	Uploader sdk.AccAddress     `json:"uploader" yaml:"uploader"`   // user who uploads the file
}

// verify interface at compile time
var _ sdk.Msg = &MsgFileUpload{}

// NewMsg<Action> creates a new Msg<Action> instance
func NewMsgUpload(fileHash string, from sdk.AccAddress, reporter stratos.SdsAddress, uploader sdk.AccAddress) MsgFileUpload {
	return MsgFileUpload{
		FileHash: fileHash,
		From:     from,
		Reporter: reporter,
		Uploader: uploader,
	}
}

// nolint
func (msg MsgFileUpload) Route() string { return RouterKey }
func (msg MsgFileUpload) Type() string  { return ConstFileUpload }
func (msg MsgFileUpload) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.From}
}

// GetSignBytes gets the bytes for the message signer to sign on
func (msg MsgFileUpload) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic validity check for the AnteHandler
func (msg MsgFileUpload) ValidateBasic() error {
	if msg.Reporter.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "missing address of tx reporter")
	}
	if msg.Uploader.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "missing address of file uploader")
	}
	if len(msg.FileHash) == 0 {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidRequest, "missing file hash")
	}
	return nil
}

type MsgPrepay struct {
	Sender sdk.AccAddress `json:"sender" yaml:"sender"` // sender of tx
	Coins  sdk.Coins      `json:"coins" yaml:"coins"`   // coins to send
}

// verify interface at compile time
var _ sdk.Msg = &MsgPrepay{}

// NewMsg<Action> creates a new Msg<Action> instance
func NewMsgPrepay(sender sdk.AccAddress, coins sdk.Coins) MsgPrepay {
	return MsgPrepay{
		Sender: sender,
		Coins:  coins,
	}
}

// nolint
func (msg MsgPrepay) Route() string { return RouterKey }
func (msg MsgPrepay) Type() string  { return ConstSdsPrepay }
func (msg MsgPrepay) GetSigners() []sdk.AccAddress {
	return []sdk.AccAddress{msg.Sender}
}

// GetSignBytes gets the bytes for the message signer to sign on
func (msg MsgPrepay) GetSignBytes() []byte {
	bz := ModuleCdc.MustMarshalJSON(msg)
	return sdk.MustSortJSON(bz)
}

// ValidateBasic validity check for the AnteHandler
func (msg MsgPrepay) ValidateBasic() error {
	if msg.Sender.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidAddress, "missing sender address")
	}
	if msg.Coins.Empty() {
		return sdkerrors.Wrap(sdkerrors.ErrInvalidCoins, "missing coins to send")
	}
	return nil
}
