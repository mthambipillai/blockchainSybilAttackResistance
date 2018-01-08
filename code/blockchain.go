package main

import(
	"crypto/rsa"
	"crypto/sha256"
	"time"
	"encoding/binary"
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"math/big"
)

const difficulty = 16
const expiration = time.Second*20

type BlockChain struct{
	BlocksPerNodeID map[uint64]*Block
	OrderedNodesIDs []uint64
	LastBlock *Block
}

type Block struct{
	NodeID 			uint64           // for sybil attack resistance protocol
	Timestamp 		time.Time
	PubKey			BCPublicKey
	Nonce			[]byte
	PreviousHash	[]byte
}

//had to create this because protobuf can't encode math/big.Int 
type BCPublicKey struct{
	N []byte
	E int
}

func bcPubKeyTorsaPubKey(pub BCPublicKey) rsa.PublicKey{
	var N big.Int
	N.GobDecode(pub.N)
	return rsa.PublicKey{&N, pub.E}
}

func rsaPubKeyTobcPubKey(pub rsa.PublicKey) BCPublicKey{
	nBytes,_ := pub.N.GobEncode()
	return BCPublicKey{nBytes, pub.E}
}

func pubKeyToBytes(pub BCPublicKey)[]byte{
	eBytes := make([]byte, 4)
    binary.LittleEndian.PutUint32(eBytes, uint32(pub.E))
    return append(pub.N, eBytes...)
}

func (b *Block) getBytes()[]byte{
	nodeIDBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nodeIDBytes, b.NodeID)
	timeBytes,_ := b.Timestamp.GobEncode()

	nodeTime := append(nodeIDBytes,timeBytes...)
    publicKeyBytes := pubKeyToBytes(b.PubKey)

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
		fmt.Println("Adds block for id",b.NodeID)
		bc.BlocksPerNodeID[b.NodeID] = b
		bc.LastBlock = b
		bc.OrderedNodesIDs = bc.addNodeIDToList(b.NodeID)
		return true
	}else{
		return false
	}
}

func (bc *BlockChain) addNodeIDToList(id uint64) []uint64{
	res := make([]uint64,0)
	for _,n := range(bc.OrderedNodesIDs){
		if(n!=id){
			res = append(res, n)
		}
	}
	return append(res, id)
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

func (bc *BlockChain) initGenesis(id uint64, pub *rsa.PublicKey){
	fmt.Println("Start the blockchain for id.",id)
	bc.BlocksPerNodeID = make(map[uint64]*Block)
	var b *Block = nil
	prevHash := []byte("random string because prevHash cannot be nil")
	for b==nil{
		b = mineBlock(id, time.Now(), pub, prevHash)
	}
	fmt.Println("Done mining the genesis block.")
	bc.BlocksPerNodeID[id] = b
	bc.LastBlock = b
	bc.OrderedNodesIDs = []uint64{id}
}

func (bc *BlockChain) nextNodeID()uint64{
	i := uint64(1)
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

func (bc *BlockChain) print(){
	nodesIDs := make([]string,0)
	for _,nodeID := range(bc.OrderedNodesIDs){
		nodesIDs = append(nodesIDs, strconv.Itoa(int(nodeID)))
	}
	s := strings.Join(nodesIDs, " <-- ")
	fmt.Println(s)
}

func (bc *BlockChain) nonExpiredBlockChain() *BlockChain{
	expired := true
	i:=0
	for expired{
		if(!bc.BlocksPerNodeID[bc.OrderedNodesIDs[i]].expired()){
			fmt.Println(i)
			fmt.Println("not expired")
			expired = false
		}else{
			fmt.Println(i)
			fmt.Println("expired")
		}
		i=i+1
	}
	validNodesIDs := bc.OrderedNodesIDs[(i-1):]
	validBlocks := make(map[uint64]*Block)
	for _,nodeID := range(validNodesIDs){
		validBlocks[nodeID] = bc.BlocksPerNodeID[nodeID]
	}

	return &BlockChain{validBlocks, validNodesIDs, bc.LastBlock}
}

func (bc *BlockChain) checkIntegrity() bool{
	var prevHash []byte
	for _,id := range(bc.OrderedNodesIDs){
		b := bc.BlocksPerNodeID[id]
		if(!b.isValid()){
			fmt.Println("Block is not valid")
			return false
		}
		if(prevHash!=nil){
			if(bytes.Compare(b.PreviousHash, prevHash)!=0){
				return false
			}
			prevHash = b.hash()
		}
	}
	return true
}

func mineBlock(id uint64, timestamp time.Time, pub *rsa.PublicKey, previousHash []byte) *Block{
	i := 0
	start := time.Now()
	trial := &Block{}
	bcPubKey := rsaPubKeyTobcPubKey(*pub)
	for{
		nonceBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(nonceBytes, uint64(i))
		trial = &Block{id, timestamp, bcPubKey, nonceBytes, previousHash}
		if(trial.isValid()){
			return trial
		}
		i=i+1
		if(i%1000000==0){
			fmt.Println("Tried "+strconv.Itoa(i/1000000)+" million different nonces.")
		}
		if(time.Since(start).Nanoseconds() > expiration.Nanoseconds()){
			return nil
		}
	}
	return nil
}

func (bc *BlockChain) printChain(){
	for k,v := range bc.BlocksPerNodeID{
		fmt.Println("Known Blocks: ",k," ",v)
	}

}