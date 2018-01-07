package main

import (
	"net"
	"fmt"
	"sync"
	"time"
)

const attempts = 2

type InactiveNode struct{
	IPAddr string
	NodeID uint64
}

type IPMatchNodeID struct{
	IPAddr string
	NodeID uint64
}

type CheckNeighbors struct{
	neighbors		map[string]int		// IP Address -> counter, that counts how many times node was unresponsive
	activeNeighbors map[string]bool		// nodeID of blockchain -> true/false if is active or not respectively
	matchIPtoID		map[string]uint64
	mu1				sync.RWMutex 		// for matches
	mu2				sync.RWMutex		// for active neighbors
	PResponseChan	chan *IPMatchNodeID
}

func createCheckNeighborsProtocol(PResp chan *IPMatchNodeID) *CheckNeighbors{
	cN := CheckNeighbors{
		neighbors:			make(map[string]int),
		activeNeighbors:	make(map[string]bool),
		matchIPtoID:		make(map[string]uint64),
		PResponseChan:		PResp,
	}
	return &cN
}

// every 5 seconds check for the neighbor nodes if they are still active
func (srh *SybilResistanceHandler) searchForInactiveNodes(){
	go func() {
		for{
			select{
				case <- time.After(2*time.Second):
					srh.checkNeighbors()
					break
			}
		}
	}()
}

func (srh *SybilResistanceHandler) checkNeighbors() {
	for _, peer := range srh.ps.peers{
		_, ok := srh.cn.matchIPtoID[peer.address.String()]
		if ok{
			counter, ok := srh.cn.neighbors[peer.address.String()]
			if !ok{
				srh.cn.neighbors[peer.address.String()] = 0
				if !srh.nodesAlive(peer.address){
					srh.cn.neighbors[peer.address.String()] = 1
				}
			}else{
				if counter < 2 {
					if !srh.nodesAlive(peer.address){
						srh.cn.neighbors[peer.address.String()] = counter + 1
					}else{
						srh.cn.neighbors[peer.address.String()] = 0
					}
				}
			}
			if srh.cn.neighbors[peer.address.String()] == attempts{
				srh.cn.activeNeighbors[peer.address.String()] = false
				srh.createInactivePacket(srh.cn.matchIPtoID[peer.address.String()], peer.address.String())
			}else{
				srh.cn.activeNeighbors[peer.address.String()] = true
			}
		}
	}
}

func (srh *SybilResistanceHandler) broadcastBlockChain(from *net.UDPAddr){
	for _,v := range srh.ps.LocalChain.OrderedNodesIDs{
		fmt.Println("Known Nodes: ",v)
	}
	srh.ps.LocalChain.printChain()
}

// in case a node is down, remove it from active nodes
func (srh *SybilResistanceHandler) nodesAlive(node *net.UDPAddr) bool{
	_, err := net.DialUDP("udp",nil, node)
	if err != nil {
		if _, ok := err.(net.Error); ok {
			fmt.Println("Couldn't connect to node",node)
		}
		return false
	}
	return true
}

func (srh *SybilResistanceHandler) updateMapWithMatches(){
	for {
		presp :=  <- srh.cn.PResponseChan
		srh.cn.mu1.Lock()
		_, ok := srh.cn.matchIPtoID[presp.IPAddr]
		if !ok{
			srh.cn.matchIPtoID[presp.IPAddr] = presp.NodeID
		}
		srh.cn.mu1.Unlock()
	}
}

func (srh *SybilResistanceHandler) createInactivePacket(NodeID uint64, Ip string) {
	fmt.Println("DOWNNNN:",Ip)
	Inactiv := &InactiveNode{IPAddr:Ip, NodeID:NodeID}
	msg := &GossipPacket{InNode:Inactiv, DigitalSign : srh.signDocument(srh.createHash(NodeID))}
	fmt.Println(msg)
	//TODO broadcast to all remainder neighbors
	//srh.ps.send(msg, dest)
}

func (srh *SybilResistanceHandler) handleInactivePacket(packet InactiveNode){

}


//TODO count times to present some evaluation metrics
//TODO update sybil resistance protocol with checkneighbors struct