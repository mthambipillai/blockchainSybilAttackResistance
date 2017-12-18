package main

import(
	"crypto/rsa"
	"crypto/sha256"
	"time"
	"encoding/binary"
	"bytes"
	"fmt"
	"math"
)

const difficulty = 5
const expiration = time.Second*60

type BlockChain struct{
	BlocksPerNodeID map[uint64]*Block
	LastBlock *Block
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

func (b *Block) hash() []byte{
    hasher := sha256.New()
	hasher.Write(b.getBytes())
	return hasher.Sum(nil)
}

func (b *Block) expired() bool{
	elapsed := time.Since(b.Timestamp)
    return elapsed.Nanoseconds() > expiration.Nanoseconds()
}

func (b *Block) isValid() bool{
	if(b.expired()){
		return false
	}
	nbZeroBytes := int(math.Ceil(difficulty/8.0))
	h := b.hash()[0:nbZeroBytes]
	for i:=0;i<nbZeroBytes-1;i=i+1{
		if(h[i]!=0){
			return false
		}
	}
	remainingNbBits := difficulty - (nbZeroBytes-1)*8
	lastByte := fmt.Sprintf("%08b",int64(h[nbZeroBytes-1]))
	for i:=0;i<remainingNbBits;i=i+1{
		if(string(lastByte[i])!="0"){
			return false
		}
	}
	return true
}

func (bc *BlockChain) addBlock(b *Block) bool{
	matchesLast := false
	if(bc.LastBlock==nil){
		matchesLast = true
	}else{
		matchesLast = bytes.Compare(b.PreviousHash, bc.LastBlock.hash())==0
	}
	if(b.isValid() && matchesLast){
		bc.BlocksPerNodeID[b.NodeID] = b
		bc.LastBlock = b
		return true
	}else{
		return false
	}
}

func (bc *BlockChain) containsValidNodeID(id uint64) bool{
	if(bc.BlocksPerNodeID==nil){
		return false
	}
	b,ok := bc.BlocksPerNodeID[id]
	if(ok){
		return b.isValid()
	}
	return false
}

func (bc *BlockChain) getLastHash() []byte{
	return bc.LastBlock.hash()
}

func (bc *BlockChain) containsBlock(b *Block) bool{
	if(bc.BlocksPerNodeID==nil){
		return false
	}
	_,ok := bc.BlocksPerNodeID[b.NodeID]
	return ok
}

func (bc *BlockChain) initGenesis(id uint64, pub rsa.PublicKey){
	fmt.Println("Start the blockchain.")
	bc.BlocksPerNodeID = make(map[uint64]*Block)
	var b *Block = nil
	for b==nil{
		b = mineBlock(id,time.Now(),pub,nil)
	}
	fmt.Println("Done mining the genesis block.")
	bc.BlocksPerNodeID[id] = b
	bc.LastBlock = b
}

func (bc *BlockChain) nextNodeID()uint64{
	i := uint64(0)
	for{
		b,ok := bc.BlocksPerNodeID[i]
		if(!ok){
			return i
		}
		if(!b.isValid()){
			return i
		}
		i=i+1
	}
	return i
}

func mineBlock(id uint64, timestamp time.Time, pub rsa.PublicKey, previousHash []byte) *Block{
	i := 0
	start := time.Now()
	trial := &Block{}
	for{
		nonceBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(nonceBytes, uint64(i))
		trial = &Block{id, timestamp, pub, nonceBytes, previousHash}
		if(trial.isValid()){
			return trial
		}
		i=i+1
		if(i%1000==0){
			fmt.Println("Tried "+string(i)+" different nonces.")
		}
		if(time.Since(start).Nanoseconds() > expiration.Nanoseconds()){
			return nil
		}
	}
	return nil
}