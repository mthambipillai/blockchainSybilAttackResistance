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
		srh.cn.mu1.Lock()
		val, ok := srh.cn.activeNeighbors[from.String()]
		srh.cn.mu1.Unlock()
		if ok{
			if val{                // if is active
				srh.cn.MsgRecv[from.String()] <- gp.NodeID
				//fmt.Println("Received from active",from,gp.NodeID)
				if gp.InNode != nil && gp.DigitalSign != nil{
					if srh.validatePeer(gp.DigitalSign,gp.NodeID){
						srh.cn.mu1.Lock()
						_, ok := srh.cn.activeNeighbors[gp.InNode.IPAddr]
						if ok{
							srh.cn.activeNeighbors[gp.InNode.IPAddr] = false						}
						srh.cn.mu1.Unlock()
					}else{
						return false
					}
				}
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