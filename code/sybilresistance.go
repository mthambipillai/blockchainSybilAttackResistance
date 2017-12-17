package main

type SybilResistanceHandler struct{
	MyID uint64
	localChain *BlockChain
	ps *PuzzlesState
}

func (srh *SybilResistanceHandler) handleGossipPacket(gp *GossipPacket){

}