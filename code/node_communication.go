package main

import (
	"net"
	"fmt"
	"sync"
	"time"
	"strings"
)

const attempts = 2

type InactiveNode struct{
	IPAddr string
	//NodeID uint64
}

type IPMatchNodeID struct{
	IPAddr string
	NodeID uint64
}

type CheckNeighbors struct{
	neighbors		map[string]int		// IP Address -> counter, that counts how many times node was unresponsive
	activeNeighbors map[string]bool		// nodeID of blockchain -> true/false if is active or not respectively
	//matchIPtoID		map[string]uint64
	mu1				sync.RWMutex 		// for matches
	mu2				sync.RWMutex		// for active neighbors
	PResponseChan	chan *IPMatchNodeID
	MsgRecv			map[string] chan uint64
}

func createCheckNeighborsProtocol(PResp chan *IPMatchNodeID, MsgReceival map[string] chan uint64) *CheckNeighbors{
	cN := CheckNeighbors{
		neighbors:			make(map[string]int),
		activeNeighbors:	make(map[string]bool),
		//matchIPtoID:		make(map[string]uint64),
		PResponseChan:		PResp,
		MsgRecv:			MsgReceival,
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
		_, ok := srh.cn.activeNeighbors[peer.address.String()]
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
				srh.createInactivePacket(peer.address.String())
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
	_, err := net.Dial("udp", node.String())
	if err != nil {
		fmt.Println("ERRORR")
		if _, ok := err.(net.Error); ok {
			fmt.Println("Couldn't connect to node",node)
		}
		return false
	}
	return true
}

func (srh *SybilResistanceHandler) initMap(peer string){
	srh.cn.mu1.Lock()
	srh.cn.MsgRecv[peer] = make(chan uint64)
	//srh.cn.matchIPtoID[presp.IPAddr] = presp.NodeID
	srh.cn.activeNeighbors[peer] = true
	srh.cn.neighbors[peer] = 0
	srh.cn.mu1.Unlock()
	go srh.identifyInactiveNodes(peer)
}

func (srh *SybilResistanceHandler) updateMapWithMatches(){
	for {
		presp :=  <- srh.cn.PResponseChan
		srh.cn.mu1.Lock()
		_, ok := srh.cn.activeNeighbors[presp.IPAddr]
		if !ok{
			srh.cn.MsgRecv[presp.IPAddr] = make(chan uint64)
			//srh.cn.matchIPtoID[presp.IPAddr] = presp.NodeID
			srh.cn.activeNeighbors[presp.IPAddr] = true
			srh.cn.neighbors[presp.IPAddr] = 0
		}
		go srh.identifyInactiveNodes(presp.IPAddr)
		srh.cn.mu1.Unlock()
	}
}

func (srh *SybilResistanceHandler) createInactivePacket(Ip string) {
	Inactiv := &InactiveNode{IPAddr:Ip}
	msg := &GossipPacket{InNode:Inactiv, DigitalSign : srh.signDocument(srh.createHash(Ip))}
	srh.broadcastInactivePacket(msg)
}

func (srh *SybilResistanceHandler) handleInactivePacket(packet InactiveNode){

}

func (srh *SybilResistanceHandler) identifyInactiveNodes(k string){
	c := 0
	for{
		srh.cn.mu1.Lock()
		if srh.cn.activeNeighbors[k] {
			srh.cn.mu1.Unlock()
			select {
				case <-time.After(30 * time.Second):
					c = c + 1
					if c == attempts {
						fmt.Println("Node disappeared", k)
						srh.cn.mu1.Lock()
						srh.cn.activeNeighbors[k] = false
						srh.cn.mu1.Unlock()
						srh.createInactivePacket(k)
						return
					}
					break
				case <-srh.cn.MsgRecv[k]:
					c = 0
					srh.cn.mu1.Lock()
					srh.cn.activeNeighbors[k] = true
					srh.cn.mu1.Unlock()
					break
			}
		}
	}
}

func (srh *SybilResistanceHandler) broadcastInactivePacket(msg *GossipPacket){
	for _,peer := range(srh.ps.peers){
		if strings.Compare(peer.address.String(),msg.InNode.IPAddr) != 0{
			fmt.Println("Send Inactive Packet to "+peer.address.String())
			srh.ps.send(msg, peer.address)
		}
	}
}