package main

import(
	"crypto/rsa"
	"crypto/sha256"
	"time"
	"encoding/binary"
)

type BlockChain struct{

}

type Block struct{
	NodeID 			uint64
	Timestamp 		time.Time
	PublicKey		rsa.PublicKey
	Nonce			[]byte
	PreviousHash	[]byte
}

func (b *Block) getBytes()[]byte{
	nodeIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nodeIDBytes, b.NodeID)
	timeBytes,_ := b.Timestamp.GobEncode()

	nodeTime := append(nodeIDBytes,timeBytes...)
	
	nBytes,_ := b.PublicKey.N.GobEncode()
	eBytes := make([]byte, 4)
    binary.LittleEndian.PutUint32(eBytes, uint32(b.PublicKey.E))
    publicKeyBytes := append(nBytes, eBytes...)

    nodeTimePubKey := append(nodeTime, publicKeyBytes...)
    nodeTimePubKeyNonce := append(nodeTimePubKey, b.Nonce...)
    return append(nodeTimePubKeyNonce, b.PreviousHash...)
}

func (b *Block) hash()[]byte{
    hasher := sha256.New()
	hasher.Write(b.getBytes())
	return hasher.Sum(nil)
}