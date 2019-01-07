package cs

import (
	"bytes"
	"io"

	"github.com/elastos/Elastos.ELA.Arbiter/config"
	"github.com/elastos/Elastos.ELA.Arbiter/rpc"

	"github.com/elastos/Elastos.ELA/common"
	"github.com/elastos/Elastos.ELA/core/types"
)

type IllegalDistributedContent struct {
	Evidence *types.SidechainIllegalData

	hash *common.Uint256
}

func (i *IllegalDistributedContent) Check(clientFunc interface{}) error {
	// todo check IllegalType, Height, and data hashes from sidechain rpc
	return nil
}

func (i *IllegalDistributedContent) CurrentBlockHeight() (uint32, error) {
	return i.Evidence.Height, nil
}

func (i *IllegalDistributedContent) Deserialize(r io.Reader) error {
	return i.Evidence.Deserialize(r)
}

func (i *IllegalDistributedContent) DeserializeUnsigned(r io.Reader) error {
	return i.Evidence.DeserializeUnsigned(r)
}

func (i *IllegalDistributedContent) Hash() common.Uint256 {
	if i.hash == nil {
		buf := new(bytes.Buffer)
		i.SerializeUnsigned(buf)
		hash := common.Uint256(common.Sha256D(buf.Bytes()))
		i.hash = &hash
	}
	return *i.hash
}

func (i *IllegalDistributedContent) InitSign(newSign []byte) error {
	i.Evidence.Signs = [][]byte{newSign}
	return nil
}

func (i *IllegalDistributedContent) MergeSign(newSign []byte, targetCodeHash *common.Uint160) (int, error) {
	i.Evidence.Signs = append(i.Evidence.Signs, newSign)
	return len(i.Evidence.Signs), nil
}

func (i *IllegalDistributedContent) Serialize(w io.Writer) error {
	return i.Evidence.Serialize(w)
}

func (i *IllegalDistributedContent) SerializeUnsigned(w io.Writer) error {
	return i.Evidence.SerializeUnsigned(w)
}

func (i *IllegalDistributedContent) Submit() error {
	var err error
	buf := new(bytes.Buffer)
	if err = i.Evidence.Serialize(buf); err != nil {
		return err
	}

	content := common.BytesToHexString(buf.Bytes())
	if _, err = rpc.CallAndUnmarshalResponse("submitsidechainillegaldata",
		rpc.Param("illegaldata", content), config.Parameters.MainNode.Rpc); err != nil {
		return err
	}
	return nil
}
