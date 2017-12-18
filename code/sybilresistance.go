package main

import(
	"net"
)

type SybilResistanceHandler struct{
	ps *PuzzlesState
	//TODO add a state structure for part2
}

//returns whether the gossip packet is allowed or not according to the sybil resistance protocol
func (srh *SybilResistanceHandler) handleGossipPacket(gp *GossipPacket, from *net.UDPAddr)bool{
	if(gp.NodeID!=srh.ps.MyID && srh.ps.LocalChain.containsValidNodeID(gp.NodeID)){
		//TODO : handle this with the part 2 structure
		return true
	}else{
		srh.ps.handleJoining(from)
		return false
	}
}