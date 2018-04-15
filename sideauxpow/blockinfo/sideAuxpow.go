package blockinfo

import (
	"io"

	. "github.com/elastos/Elastos.ELA.Arbiter/common"
	"github.com/elastos/Elastos.ELA.Arbiter/common/serialization"
	"github.com/elastos/Elastos.ELA.Arbiter/core/transaction"
)

type SideAuxPow struct {
	SideAuxMerkleBranch []Uint256
	SideAuxMerkleIndex  int
	SideAuxBlockTx      transaction.Transaction
	MainBlockHeader     Blockdata
}

func (sap *SideAuxPow) Serialize(w io.Writer) error {
	err := sap.SideAuxBlockTx.Serialize(w)
	if err != nil {
		return err
	}

	count := uint64(len(sap.SideAuxMerkleBranch))
	err = serialization.WriteVarUint(w, count)
	if err != nil {
		return err
	}

	for _, pcbm := range sap.SideAuxMerkleBranch {
		_, err = pcbm.Serialize(w)
		if err != nil {
			return err
		}
	}
	idx := uint32(sap.SideAuxMerkleIndex)
	err = serialization.WriteUint32(w, idx)
	if err != nil {
		return err
	}

	err = sap.MainBlockHeader.Serialize(w)
	if err != nil {
		return err
	}
	return nil
}

func (sap *SideAuxPow) Deserialize(r io.Reader) error {
	err := sap.SideAuxBlockTx.Deserialize(r)
	if err != nil {
		return err
	}

	count, err := serialization.ReadVarUint(r, 0)
	if err != nil {
		return err
	}

	sap.SideAuxMerkleBranch = make([]Uint256, count)
	for i := uint64(0); i < count; i++ {
		temp := Uint256{}
		err = temp.Deserialize(r)
		if err != nil {
			return err
		}
		sap.SideAuxMerkleBranch[i] = temp

	}

	temp, err := serialization.ReadUint32(r)
	if err != nil {
		return err
	}
	sap.SideAuxMerkleIndex = int(temp)

	err = sap.MainBlockHeader.Deserialize(r)
	if err != nil {
		return err
	}

	return nil
}