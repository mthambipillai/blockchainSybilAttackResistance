package main

import(
	"net"
)

type SybilResistanceHandler struct{
	ps *PuzzlesState
	cn *checkNeighbors
}

//returns whether the gossip packet is allowed or not according to the sybil resistance protocol
func (srh *SybilResistanceHandler) handleGossipPacket(gp *GossipPacket, from *net.UDPAddr)bool{
	if gp.NodeID!=srh.ps.MyID && srh.ps.LocalChain.containsValidNodeID(gp.NodeID){
		//TODO : handle this with the part 2 structure
		val, ok := srh.cn.activeNeighbors[from.String()]
		if val{                // if is active


		}
		if !ok{

		}

		//fmt.Println("HEREEE",from,gp.NodeID)
		//srh.broadcastBlockChain(from)
		//var signed *SignedDocument
		//hashA := srh.createHashInactive(gp.NodeID)
		//signed = srh.signDocument(hashA)
		//fmt.Println("Hash: ",signed)

		//srh.validatePeer(signed)
		return true
	}else{
		srh.ps.handleJoining(from)
		return false
	}
}