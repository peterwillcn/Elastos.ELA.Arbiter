package cs

import (
	"SPVWallet/p2p"
	"bytes"
)

import (
	. "Elastos.ELA.Arbiter/arbitration/arbitrator"
	. "Elastos.ELA.Arbiter/arbitration/base"
	. "Elastos.ELA.Arbiter/common"
	"Elastos.ELA.Arbiter/common/log"
	"errors"
)

type DistributedNodeClient struct {
	P2pCommand        string
	unsolvedProposals map[Uint256]*DistributedItem
}

func (client *DistributedNodeClient) SignProposal(transactionHash Uint256) error {
	transactionItem, ok := client.unsolvedProposals[transactionHash]
	if !ok {
		return errors.New("Can not find proposal.")
	}

	return transactionItem.Sign(ArbitratorGroupSingleton.GetCurrentArbitrator())
}

func (client *DistributedNodeClient) OnP2PReceived(peer *p2p.Peer, msg p2p.Message) {
	if msg.CMD() != client.P2pCommand {
		return
	}

	signMessage, ok := msg.(*SignMessage)
	if !ok {
		log.Warn("Unknown p2p message content.")
		return
	}

	client.OnReceivedProposal(signMessage.Content)
}

func (client *DistributedNodeClient) OnReceivedProposal(content []byte) error {
	transactionItem := &DistributedItem{}
	if err := transactionItem.Deserialize(bytes.NewReader(content)); err != nil {
		return err
	}

	hash := transactionItem.ItemContent.Hash()
	if _, ok := client.unsolvedProposals[hash]; ok {
		return errors.New("Proposal already exit.")
	}

	client.unsolvedProposals[hash] = transactionItem

	if err := client.SignProposal(hash); err != nil {
		return err
	}

	if err := client.Feedback(hash); err != nil {
		return err
	}
	return nil
}

func (client *DistributedNodeClient) Feedback(transactionHash Uint256) error {
	item, ok := client.unsolvedProposals[transactionHash]
	if !ok {
		return errors.New("Can not find proposal.")
	}

	ar := ArbitratorGroupSingleton.GetCurrentArbitrator()
	item.TargetArbitratorPublicKey = ar.GetPublicKey()

	programHash, err := StandardAcccountPublicKeyToProgramHash(item.TargetArbitratorPublicKey)
	if err != nil {
		return err
	}
	item.TargetArbitratorProgramHash = programHash

	messageReader := new(bytes.Buffer)
	err = item.Serialize(messageReader)
	if err != nil {
		return errors.New("Send complaint failed.")
	}

	client.sendBack(messageReader.Bytes())
	return nil
}

func (client *DistributedNodeClient) sendBack(message []byte) {
	P2PClientSingleton.Broadcast(&SignMessage{
		Command: client.P2pCommand,
		Content: message,
	})
}