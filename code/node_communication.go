package main

import (
	"net"
	"fmt"
	"net/url"
	"sync"
)

const attempts = 2

type InactiveNode struct{
	nodeID uint64
}

type checkNeighbors struct{
	neighbors		map[string]int		// IP Address -> counter, that counts how many times node was unresponsive
	activeNeighbors map[string]bool		// nodeID of blockchain -> true/false if is active or not respectively
	mu1				sync.RWMutex 		// for neighbors
	mu2				sync.RWMutex		// for active neighbors
}

func (srh *SybilResistanceHandler) checkNeighbors(gossiper Gossiper) {
	for _, peer := range srh.ps.peers{
		counter, ok := srh.cn.neighbors[peer.address.String()]
		if !ok{
			srh.cn.neighbors[peer.address.String()] = 0
			if !srh.nodesAlive(peer.address.String()){
				srh.cn.neighbors[peer.address.String()] = 1
			}
		}else{
			if !srh.nodesAlive(peer.address.String()){
				srh.cn.neighbors[peer.address.String()] = counter + 1
			}else{
				srh.cn.neighbors[peer.address.String()] = 0
			}
		}
		if srh.cn.neighbors[peer.address.String()] == attempts{
			srh.cn.activeNeighbors[peer.address.String()] = false
		}else{
			srh.cn.activeNeighbors[peer.address.String()] = true
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
func (srh *SybilResistanceHandler) nodesAlive(node string) bool{
	url, err1 := url.Parse(node)
	if err1 != nil {
		fmt.Print("Failed parsing URL (%v)", err1)
		return false
	}
	_, err := net.Dial("tcp", url.Host)
	if err != nil {
		return false
		if _, ok := err.(net.Error); ok {
			fmt.Println("Couldn't connect to node",node)
		}
	}
	return true
}

//TODO update GossipPacket with digital signatures
//TODO update with InactiveNode
//TODO count times to present some evaluation metrics
//TODO update sybil resistance protocol with checkneighbors struct