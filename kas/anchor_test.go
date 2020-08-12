// Copyright 2020 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package kas

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/crypto"
	"github.com/klaytn/klaytn/kas/mocks"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"
	"strconv"
	"testing"
)

var (
	errTest = errors.New("test error")
)

func testAnchorData() *types.AnchoringDataInternalType0 {
	return &types.AnchoringDataInternalType0{
		BlockHash:     common.HexToHash("0"),
		TxHash:        common.HexToHash("1"),
		ParentHash:    common.HexToHash("2"),
		ReceiptHash:   common.HexToHash("3"),
		StateRootHash: common.HexToHash("4"),
		BlockNumber:   big.NewInt(5),
		BlockCount:    big.NewInt(6),
		TxCount:       big.NewInt(7),
	}
}

func TestExampleSendRequest(t *testing.T) {
	url := "http://anchor.wallet-api.dev.klaytn.com/v1/anchor"
	xkrn := "krn:1001:anchor:test:operator-pool:op1"
	user := "78ab9116689659321aaf472aa154eac7dd7a99c6"
	pwd := "403e0397d51a823cd59b7edcb212788c8599dd7e"

	operator := common.StringToAddress("0x1552F52D459B713E0C4558e66C8c773a75615FA8")

	// Anchor Data
	anchorData := testAnchorData()

	kasConfig := &KASConfig{
		Url:          url,
		Xkrn:         xkrn,
		User:         user,
		Pwd:          pwd,
		Operator:     operator,
		Anchor:       true,
		AnchorPeriod: 1,
	}

	kasAnchor := NewKASAnchor(kasConfig, nil, nil)

	payload := dataToPayload(anchorData)
	res, err := kasAnchor.sendRequest(payload)
	assert.NoError(t, err)

	result, err := json.Marshal(res)
	assert.NoError(t, err)

	t.Log(string(result))
}

func TestSendRequest(t *testing.T) {
	config := KASConfig{}
	anchor := NewKASAnchor(&config, nil, nil)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockHTTPClient(ctrl)
	anchor.client = m

	anchorData := testAnchorData()
	pl := dataToPayload(anchorData)

	// OK case
	{
		expectedRes := http.Response{Status: strconv.Itoa(http.StatusOK)}
		expectedRespBody := respBody{
			Code: 0,
		}
		bodyBytes, _ := json.Marshal(expectedRespBody)
		expectedRes.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))

		m.EXPECT().Do(gomock.Any()).Times(1).Return(&expectedRes, nil)
		resp, err := anchor.sendRequest(pl)

		assert.NoError(t, err)
		assert.Equal(t, expectedRespBody.Code, resp.Code)
	}

	// Error case
	{
		m.EXPECT().Do(gomock.Any()).Times(1).Return(nil, errTest)
		resp, err := anchor.sendRequest(pl)

		assert.Error(t, errTest, err)
		assert.Nil(t, resp)
	}
}

func TestDataToPayload(t *testing.T) {
	anchorData := testAnchorData()
	pl := dataToPayload(anchorData)
	assert.Equal(t, anchorData.BlockNumber.String(), pl.Id)
	assert.Equal(t, *anchorData, pl.AnchoringDataInternalType0)
}

func TestBlockToAnchoringDataInternalType0(t *testing.T) {
	testBlockToAnchoringDataInternalType0(t, 1)
	testBlockToAnchoringDataInternalType0(t, 7)
	testBlockToAnchoringDataInternalType0(t, 100)
}

func testBlockToAnchoringDataInternalType0(t *testing.T, period uint64) {
	random := rand.New(rand.NewSource(0))

	config := KASConfig{
		Anchor:       true,
		AnchorPeriod: period,
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	bc := mocks.NewMockBlockChain(ctrl)

	anchor := NewKASAnchor(&config, nil, bc)
	testBlkN := uint64(100)
	pastCnt := [100]uint64{}
	txCnt := uint64(0)

	for blkNum := uint64(0); blkNum < testBlkN; blkNum++ {
		// Gen random block
		header := &types.Header{Number: big.NewInt(int64(blkNum))}
		block := types.NewBlockWithHeader(header)
		txNum := random.Uint64() % 10
		txs, _ := genTransactions(txNum)
		body := &types.Body{Transactions: txs}
		block = block.WithBody(body.Transactions)

		// update blockchain mock
		bc.EXPECT().GetBlockByNumber(blkNum).Return(block).AnyTimes()

		// call target func
		result := anchor.blockToAnchoringDataInternalType0(block)

		// calc expected value
		txCnt -= pastCnt[blkNum%anchor.kasConfig.AnchorPeriod]
		pastCnt[blkNum%anchor.kasConfig.AnchorPeriod] = txNum
		txCnt += txNum

		// compare result
		assert.Equal(t, txCnt, result.TxCount.Uint64(), "blkNum:%v", blkNum)
	}
}

func genTransactions(n uint64) (types.Transactions, error) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)
	signer := types.NewEIP155Signer(big.NewInt(18))

	txs := types.Transactions{}

	for i := uint64(0); i < n; i++ {
		tx, _ := types.SignTx(
			types.NewTransaction(0, addr,
				big.NewInt(int64(n)), 0, big.NewInt(int64(n)), nil), signer, key)

		txs = append(txs, tx)
	}

	return txs, nil
}