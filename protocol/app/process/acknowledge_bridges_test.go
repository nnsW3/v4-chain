package process_test

import (
	"errors"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dydxprotocol/v4-chain/protocol/app/process"
	"github.com/dydxprotocol/v4-chain/protocol/mocks"
	"github.com/dydxprotocol/v4-chain/protocol/testutil/constants"
	"github.com/dydxprotocol/v4-chain/protocol/testutil/encoding"
	keepertest "github.com/dydxprotocol/v4-chain/protocol/testutil/keeper"
	"github.com/dydxprotocol/v4-chain/protocol/x/bridge/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDecodeAcknowledgeBridgesTx(t *testing.T) {
	encodingCfg := encoding.GetTestEncodingCfg()
	txBuilder := encodingCfg.TxConfig.NewTxBuilder()

	// Valid.
	validMsgTxBytes := constants.MsgAcknowledgeBridges_Ids0_1_Height0_TxBytes

	// Duplicate.
	_ = txBuilder.SetMsgs(
		constants.MsgAcknowledgeBridges_Id0_Height0,
		constants.MsgAcknowledgeBridges_Id0_Height0,
	)
	duplicateMsgTxBytes, _ := encodingCfg.TxConfig.TxEncoder()(txBuilder.GetTx())

	// Incorrect type.
	incorrectMsgTxBytes := constants.ValidMsgUpdateMarketPricesTxBytes

	tests := map[string]struct {
		txBytes []byte

		expectedErr error
		expectedMsg *types.MsgAcknowledgeBridges
	}{
		"Error: decode fails": {
			txBytes:     []byte{1, 2, 3}, // invalid bytes.
			expectedErr: errors.New("tx parse error: Decoding tx bytes failed"),
		},
		"Error: empty bytes": {
			txBytes: []byte{}, // empty returns 0 msgs.
			expectedErr: errors.New("Msg Type: types.MsgAcknowledgeBridges, " +
				"Expected 1 num of msgs, but got 0: Unexpected num of msgs"),
		},
		"Error: incorrect msg len": {
			txBytes: duplicateMsgTxBytes,
			expectedErr: errors.New("Msg Type: types.MsgAcknowledgeBridges, " +
				"Expected 1 num of msgs, but got 2: Unexpected num of msgs"),
		},
		"Error: incorrect msg type": {
			txBytes: incorrectMsgTxBytes,
			expectedErr: errors.New(
				"Expected MsgType types.MsgAcknowledgeBridges, but " +
					"got *types.MsgUpdateMarketPrices: Unexpected msg type",
			),
		},
		"Valid": {
			txBytes:     validMsgTxBytes,
			expectedMsg: constants.MsgAcknowledgeBridges_Ids0_1_Height0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx, k, _, _, _, _ := keepertest.BridgeKeepers(t)
			abt, err := process.DecodeAcknowledgeBridgesTx(ctx, k, encodingCfg.TxConfig.TxDecoder(), tc.txBytes)
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
				require.Nil(t, abt)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedMsg, abt.GetMsg())
			}
		})
	}
}

func TestAcknowledgeBridgesTx_Validate(t *testing.T) {
	tests := map[string]struct {
		txBytes               []byte
		acknowledgedEventInfo types.BridgeEventInfo
		recognizedEventInfo   types.BridgeEventInfo

		expectedErr error
	}{
		"Error: bridge event ID not next to be acknowledged": {
			txBytes: constants.MsgAcknowledgeBridges_Id55_Height15_TxBytes,
			acknowledgedEventInfo: types.BridgeEventInfo{
				NextId:         54,
				EthBlockHeight: 12,
			},
			expectedErr: types.ErrBridgeIdNotNextToAcknowledge,
		},
		"Error: bridge event ID next to be acknowledged but not recognized": {
			txBytes: constants.MsgAcknowledgeBridges_Id55_Height15_TxBytes,
			acknowledgedEventInfo: types.BridgeEventInfo{
				NextId:         55,
				EthBlockHeight: 12,
			},
			recognizedEventInfo: types.BridgeEventInfo{
				NextId:         55,
				EthBlockHeight: 14,
			},
			expectedErr: types.ErrBridgeIdNotRecognized,
		},
		"Error: bridge event IDs not consecutive": {
			txBytes:               constants.MsgAcknowledgeBridges_Ids0_55_Height0_TxBytes,
			acknowledgedEventInfo: constants.AcknowledgedEventInfo_Id0_Height0,
			recognizedEventInfo: types.BridgeEventInfo{
				NextId:         56,
				EthBlockHeight: 14,
			},
			expectedErr: types.ErrBridgeIdsNotConsecutive,
		},
		"Valid: empty events": {
			txBytes:               constants.MsgAcknowledgeBridges_NoEvents_TxBytes,
			acknowledgedEventInfo: constants.AcknowledgedEventInfo_Id0_Height0,
			recognizedEventInfo:   constants.RecognizedEventInfo_Id2_Height0,
		},
		"Valid: one event": {
			txBytes:               constants.MsgAcknowledgeBridges_Id0_Height0_TxBytes,
			acknowledgedEventInfo: constants.AcknowledgedEventInfo_Id0_Height0,
			recognizedEventInfo:   constants.RecognizedEventInfo_Id2_Height0,
		},
		"Valid: two events": {
			txBytes:               constants.MsgAcknowledgeBridges_Ids0_1_Height0_TxBytes,
			acknowledgedEventInfo: constants.AcknowledgedEventInfo_Id0_Height0,
			recognizedEventInfo:   constants.RecognizedEventInfo_Id2_Height0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup.
			ctx, _, _, _, _, _ := keepertest.BridgeKeepers(t)
			mockBridgeKeeper := &mocks.ProcessBridgeKeeper{}
			mockBridgeKeeper.On("GetAcknowledgedEventInfo", mock.Anything).Return(tc.acknowledgedEventInfo)
			mockBridgeKeeper.On("GetRecognizedEventInfo", mock.Anything).Return(tc.recognizedEventInfo)

			abt, err := process.DecodeAcknowledgeBridgesTx(
				ctx,
				mockBridgeKeeper,
				constants.TestEncodingCfg.TxConfig.TxDecoder(),
				tc.txBytes,
			)
			require.NoError(t, err)

			// Run and Validate.
			err = abt.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAcknowledgeBridgesTx_GetMsg(t *testing.T) {
	tests := map[string]struct {
		txWrapper   process.AcknowledgeBridgesTx
		txBytes     []byte
		expectedMsg *types.MsgAcknowledgeBridges
	}{
		"Returns nil": {
			txWrapper: process.AcknowledgeBridgesTx{},
		},
		"Returns valid msg": {
			txBytes:     constants.MsgAcknowledgeBridges_Ids0_1_Height0_TxBytes,
			expectedMsg: constants.MsgAcknowledgeBridges_Ids0_1_Height0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var msg sdk.Msg
			if tc.txBytes != nil {
				ctx, k, _, _, _, _ := keepertest.BridgeKeepers(t)
				abt, err := process.DecodeAcknowledgeBridgesTx(ctx, k, constants.TestEncodingCfg.TxConfig.TxDecoder(), tc.txBytes)
				require.NoError(t, err)
				msg = abt.GetMsg()
			} else {
				msg = tc.txWrapper.GetMsg()
			}
			require.Equal(t, tc.expectedMsg, msg)
		})
	}
}