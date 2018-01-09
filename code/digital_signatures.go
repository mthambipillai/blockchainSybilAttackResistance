package main

import (
	"crypto/sha256"
	"crypto/rsa"
	"crypto/rand"
	"crypto"
	"fmt"
	"strconv"
)

type SignedDocument struct{
	HashedData		[]byte
	EncryptedData 	[]byte
}

// create hash value for inactive packets
func (srh *SybilResistanceHandler)createHash(nodeID string) []byte{
	bs := []byte(nodeID)
	hasher := sha256.New()
	hasher.Write(bs)
	return hasher.Sum(nil)
}

// create hash value to sign packets
func (srh *SybilResistanceHandler)createHashForInteger(nodeID uint64) []byte {
	bs := []byte(strconv.Itoa(int(nodeID)))
	hasher := sha256.New()
	hasher.Write(bs)
	return hasher.Sum(nil)
}

// create digital signature for data
func (srh *SybilResistanceHandler)signDocument(data []byte) *SignedDocument {
	sig, err := rsa.SignPKCS1v15(rand.Reader,srh.ps.privKey,crypto.SHA256,data)
	if err == nil{
		return &SignedDocument{data, sig}
	}else{
		fmt.Println("Error when signing the data to send!",err)
		return nil
	}
}

//validate if the SignedDocument is coming from a trusted peer
func (srh *SybilResistanceHandler)validatePeer(signedData *SignedDocument, NodeID uint64) bool{
	if signedData != nil{
		err:= rsa.VerifyPKCS1v15(srh.findPublicKeyForCorrespondingNode(NodeID),crypto.SHA256,signedData.HashedData,signedData.EncryptedData)
		if err == nil{
			fmt.Println("Trusted node.")
			return true
		}else {
			fmt.Println("Error from verification!")
			return false
		}
	}else{
		fmt.Println("Error when decrypting the data received!")
		return false
	}
}

func (srh *SybilResistanceHandler) findPublicKeyForCorrespondingNode(NodeID uint64) *rsa.PublicKey{
	var PublicKey rsa.PublicKey
	PublicKey =  bcPubKeyTorsaPubKey(srh.ps.LocalChain.BlocksPerNodeID[NodeID].PubKey)
	return &PublicKey
}