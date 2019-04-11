package sidechain

import (
	"bytes"
	"errors"
	"sync"

	"github.com/elastos/Elastos.ELA.Arbiter/arbitration/arbitrator"
	"github.com/elastos/Elastos.ELA.Arbiter/arbitration/base"
	"github.com/elastos/Elastos.ELA.Arbiter/config"
	"github.com/elastos/Elastos.ELA.Arbiter/log"
	"github.com/elastos/Elastos.ELA.Arbiter/rpc"
	"github.com/elastos/Elastos.ELA.Arbiter/sideauxpow"
	"github.com/elastos/Elastos.ELA.Arbiter/store"

	"github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/core/types"
	"github.com/elastos/Elastos.ELA/core/types/payload"
	"github.com/elastos/Elastos.ELA/elanet/pact"
)

type SideChainImpl struct {
	mux sync.Mutex

	Key           string
	CurrentConfig *config.SideNodeConfig
}

func (sc *SideChainImpl) GetKey() string {
	return sc.Key
}

func (sc *SideChainImpl) getCurrentConfig() *config.SideNodeConfig {
	sc.mux.Lock()
	defer sc.mux.Unlock()
	if sc.CurrentConfig == nil {
		for _, sideConfig := range config.Parameters.SideNodeList {
			if sc.GetKey() == sideConfig.GenesisBlockAddress {
				sc.CurrentConfig = sideConfig
				break
			}
		}
	}
	return sc.CurrentConfig
}

func (sc *SideChainImpl) GetExchangeRate() (float64, error) {
	con := sc.getCurrentConfig()
	if con == nil {
		return 0, errors.New("get exchange rate failed, side chain has no config")
	}
	if sc.getCurrentConfig().ExchangeRate <= 0 {
		return 0, errors.New("get exchange rate failed, invalid exchange rate")
	}

	return sc.getCurrentConfig().ExchangeRate, nil
}

func (sc *SideChainImpl) GetCurrentHeight() (uint32, error) {
	return rpc.GetCurrentHeight(sc.getCurrentConfig().Rpc)
}

func (sc *SideChainImpl) GetBlockByHeight(height uint32) (*base.BlockInfo, error) {
	return rpc.GetBlockByHeight(height, sc.getCurrentConfig().Rpc)
}

func (sc *SideChainImpl) SendTransaction(txHash *common.Uint256) (rpc.Response, error) {
	log.Info("[Rpc-sendtransactioninfo] Deposit transaction to side chain：", sc.CurrentConfig.Rpc.IpAddress, ":", sc.CurrentConfig.Rpc.HttpJsonPort)
	response, err := rpc.CallAndUnmarshalResponse("sendrechargetransaction", rpc.Param("txid", txHash.String()), sc.CurrentConfig.Rpc)
	if err != nil {
		return rpc.Response{}, err
	}
	log.Info("[Rpc-sendtransactioninfo] Deposit transaction finished")

	if response.Error != nil {
		log.Info("response: ", response.Error.Message)
	} else {
		log.Info("response:", response)
	}

	return response, nil
}

func (sc *SideChainImpl) GetAccountAddress() string {
	return sc.GetKey()
}

func (sc *SideChainImpl) OnUTXOChanged(txinfos []*base.WithdrawTx, blockHeight uint32) error {
	if len(txinfos) == 0 {
		return errors.New("[OnUTXOChanged] received txinfos, but size is 0")
	}

	var txs []*base.SideChainTransaction
	for _, txinfo := range txinfos {
		buf := new(bytes.Buffer)
		if err := txinfo.Serialize(buf); err != nil {
			return errors.New("[OnUTXOChanged] received txinfos, but have invalid tx," + err.Error())
		}

		txs = append(txs, &base.SideChainTransaction{
			TransactionHash:     txinfo.Txid.String(),
			GenesisBlockAddress: sc.GetKey(),
			Transaction:         buf.Bytes(),
			BlockHeight:         blockHeight,
		})
	}

	if err := store.DbCache.SideChainStore.AddSideChainTxs(txs); err != nil {
		return err
	}

	log.Info("[OnUTXOChanged] Find ", len(txs), "withdraw transaction, add into dbcache")
	return nil
}

func (sc *SideChainImpl) OnIllegalEvidenceFound(evidence *payload.SidechainIllegalData) error {
	arbitrator.ArbitratorGroupSingleton.GetCurrentArbitrator().BroadcastSidechainIllegalData(evidence)
	return nil
}

func (sc *SideChainImpl) StartSideChainMining() {
	if sc.CurrentConfig.PowChain {
		log.Info("[OnDutyChanged] Start side chain mining: genesis address [", sc.Key, "]")
		sideauxpow.StartSideChainMining(sc.CurrentConfig)
	} else {
		log.Debug("[StartSideChainMining] side chain is not pow chain, no need to mining")
	}
}

func (sc *SideChainImpl) SubmitAuxpow(genesishash string, blockhash string, submitauxpow string) error {
	return sideauxpow.SubmitAuxpow(genesishash, blockhash, submitauxpow)
}

func (sc *SideChainImpl) UpdateLastNotifySideMiningHeight(genesisBlockHash common.Uint256) {
	sideauxpow.UpdateLastNotifySideMiningHeight(genesisBlockHash)
}

func (sc *SideChainImpl) UpdateLastSubmitAuxpowHeight(genesisBlockHash common.Uint256) {
	sideauxpow.UpdateLastSubmitAuxpowHeight(genesisBlockHash)
}

func (sc *SideChainImpl) GetExistDepositTransactions(txs []string) ([]string, error) {
	receivedTxs, err := rpc.GetExistDepositTransactions(txs, sc.CurrentConfig.Rpc)
	if err != nil {
		return nil, err
	}
	return receivedTxs, nil
}

func (sc *SideChainImpl) GetWithdrawTransaction(txHash string) (*base.WithdrawTxInfo, error) {
	txInfo, err := rpc.GetTransactionInfoByHash(txHash, sc.CurrentConfig.Rpc)
	if err != nil {
		return nil, err
	}

	return txInfo, nil
}

func (sc *SideChainImpl) CheckIllegalEvidence(evidence *base.SidechainIllegalDataInfo) (bool, error) {
	return rpc.CheckIllegalEvidence(evidence, sc.CurrentConfig.Rpc)
}

func (sc *SideChainImpl) parseUserWithdrawTransactionInfo(txs []*base.WithdrawTx) (*base.WithdrawInfo, error) {
	result := new(base.WithdrawInfo)
	for _, tx := range txs {
		for _, withdraw := range tx.WithdrawInfo.WithdrawAssets {
			result.WithdrawAssets = append(result.WithdrawAssets, withdraw)
		}
	}
	return result, nil
}

func (sc *SideChainImpl) SendCachedWithdrawTxs() {
	log.Info("[SendCachedWithdrawTxs] start")
	defer log.Info("[SendCachedWithdrawTxs] end")

	txHashes, blockHeights, err := store.DbCache.SideChainStore.GetAllSideChainTxHashesAndHeights(sc.GetKey())
	if err != nil {
		log.Errorf("[SendCachedWithdrawTxs] %s", err.Error())
		return
	}

	if len(txHashes) == 0 {
		log.Info("No cached withdraw transaction need to send")
		return
	}

	if len(txHashes) > config.Parameters.MaxTxsPerWithdrawTx {
		txHashes = txHashes[:config.Parameters.MaxTxsPerWithdrawTx]
	}

	receivedTxs, err := rpc.GetExistWithdrawTransactions(txHashes)
	if err != nil {
		log.Errorf("[SendCachedWithdrawTxs] %s", err.Error())
		return
	}

	unsolvedTxs, _ := base.SubstractTransactionHashesAndBlockHeights(txHashes, blockHeights, receivedTxs)
	if len(unsolvedTxs) != 0 {
		err := sc.CreateAndBroadcastWithdrawProposal(unsolvedTxs)
		if err != nil {
			log.Error("[ReceiveSendLastArbiterUsedUtxos] CreateAndBroadcastWithdrawProposal failed")
		}

		log.Info("[SendCachedWithdrawTxs] Find withdraw transaction, send  mssage")
	}

	if len(receivedTxs) != 0 {
		err = store.DbCache.SideChainStore.RemoveSideChainTxs(receivedTxs)
		if err != nil {
			log.Errorf("[SendCachedWithdrawTxs] %s", err.Error())
			return
		}

		err = store.FinishedTxsDbCache.AddSucceedWithdrawTxs(receivedTxs)
		if err != nil {
			log.Errorf("[SendCachedWithdrawTxs] %s", err.Error())
			return
		}
	}
}

func (sc *SideChainImpl) CreateAndBroadcastWithdrawProposal(txnHashes []string) error {
	unsolvedTransactions, err := store.DbCache.SideChainStore.GetSideChainTxsFromHashes(txnHashes)
	if err != nil {
		return err
	}

	if len(unsolvedTransactions) == 0 {
		return nil
	}

	withdrawInfo, err := sc.parseUserWithdrawTransactionInfo(unsolvedTransactions)
	if err != nil {
		return err
	}

	currentArbitrator := arbitrator.ArbitratorGroupSingleton.GetCurrentArbitrator()

	var wTx *types.Transaction
	for i := 0; i < len(unsolvedTransactions); {
		i += 100
		targetIndex := len(txnHashes)
		if targetIndex > i {
			targetIndex = i
		}
		tx := currentArbitrator.CreateWithdrawTransaction(withdrawInfo, sc, txnHashes[:targetIndex], &arbitrator.MainChainFuncImpl{})
		if tx.GetSize() < pact.MaxBlockSize {
			wTx = tx
		}
	}

	currentArbitrator.BroadcastWithdrawProposal(wTx)
	log.Info("[CreateAndBroadcastWithdrawProposal] Transactions count: ", len(withdrawInfo.WithdrawAssets))

	return nil
}
