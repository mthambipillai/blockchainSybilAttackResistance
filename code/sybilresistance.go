package main

import(
	"net"
)

type SybilResistanceHandler struct{
	ps *PuzzlesState
	cn *CheckNeighbors
}

//returns whether the gossip packet is allowed or not according to the sybil resistance protocol
func (srh *SybilResistanceHandler) handleGossipPacket(gp *GossipPacket, from *net.UDPAddr)bool{
	if gp.NodeID!=srh.ps.MyID && srh.ps.LocalChain.containsValidNodeID(gp.NodeID){

		if gp.DigitalSign != nil{

		}

		val, ok := srh.cn.activeNeighbors[from.String()]
		if ok{
			if val{                // if is active

				//TODO HERE
				return true

			}else{
				return false
			}
		}else{

			return false
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