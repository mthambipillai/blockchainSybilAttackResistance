package main

import(
	"net"
)

type SybilResistanceHandler struct{
	MyID uint64
	localChain *BlockChain
	ps *PuzzlesState
	//TODO add a state structure for part2
}

func (srh *SybilResistanceHandler) handleGossipPacket(gp *GossipPacket, from *net.UDPAddr){
	if(srh.localChain.containsValidNodeID(gp.NodeID)){
		//TODO : handle this with the part 2 structure
	}else{
		srh.ps.sendPuzzleProposal(from)
	}
}