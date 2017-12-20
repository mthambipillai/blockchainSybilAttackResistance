package main
import(
	"time"
	"net"
	"github.com/dedis/protobuf"
	"fmt"
	"crypto/rsa"
)
type PuzzleProposal struct{
	Origin			string
	NodeID 			uint64
	Timestamp 		time.Time
	PreviousHash	[]byte
}

type PuzzleResponse struct{
	Origin 			string
	Destination		string
	CreatedBlock	*Block
}

type BlockBroadcast struct{
	Origin		string
	NewBlock	*Block
}

type BlockChainMessage struct{
	Chain 		*BlockChain
}

type PuzzlesState struct{
	MyID		uint64
	MyName		string
	privKey		*rsa.PrivateKey
	PubKey 		*rsa.PublicKey
	Joined		bool
	LocalChain	*BlockChain
	conn		*net.UDPConn
	waiting		map[string]*PuzzleProposal
	peers 		[]*Gossiper
}

func (ps *PuzzlesState) addNewGossiper(address, identifier string){
	for _,g := range(ps.peers){
		if(g.address.String()==address){
			return
		}
	}
	udpAddr, _:= net.ResolveUDPAddr("udp", address)
	g := &Gossiper{
		address: udpAddr,
		identifier: identifier,
	}

	ps.peers = append(ps.peers, g)
}


func (ps *PuzzlesState) handlePuzzleProposal(pp *PuzzleProposal, from *net.UDPAddr){
	if(!ps.Joined){
		fmt.Println("Received puzzle proposal. Start mining.")
		b := mineBlock(pp.NodeID, pp.Timestamp, ps.PubKey, pp.PreviousHash)
		fmt.Println("Done mining. Send puzzle response.")
		ps.MyID = pp.NodeID
		pr := &PuzzleResponse{ps.MyName, pp.Origin, b}
		ps.send(&GossipPacket{PResponse: pr}, from)
	}
}

func (ps *PuzzlesState) handlePuzzleResponse(pr *PuzzleResponse, from *net.UDPAddr){
	fmt.Println("Received puzzle response.")
	pp,ok := ps.waiting[from.String()]
	if(ok && pr.Destination==ps.MyName){
		if(pr.CreatedBlock.NodeID==pp.NodeID && pr.CreatedBlock.Timestamp.Equal(pp.Timestamp)){
			success := ps.LocalChain.addBlock(pr.CreatedBlock)
			if(success){
				delete(ps.waiting, from.String())
				ps.addNewGossiper(from.String(), pr.Origin)
				fmt.Println("The puzzle response is correct.")
				ps.LocalChain.print()
				ps.broadcastBlock(pr.CreatedBlock, from)
				ps.sendBlockChain(from)
			}else{
				fmt.Println("the puzzle response is incorrect.")
			}
		}	
	}
}

func (ps *PuzzlesState) broadcastBlock(b *Block, from *net.UDPAddr){
	bb := &BlockBroadcast{ps.MyName, b}
	msg := &GossipPacket{BBroadcast : bb}
	for _,peer := range(ps.peers){
		if(peer.address.String()!=from.String()){
			fmt.Println("Send block to "+peer.address.String())
			ps.send(msg, peer.address)
		}
	}
}

func (ps *PuzzlesState) sendBlockChain(dest *net.UDPAddr){
	bcm := &BlockChainMessage{ps.LocalChain}
	msg := &GossipPacket{BChain : bcm}
	ps.send(msg, dest)
}

func (ps *PuzzlesState) handleBlockChain(bcm *BlockChainMessage, from *net.UDPAddr){
	if(!ps.Joined){
		ps.LocalChain = bcm.Chain
		ps.Joined = true
		fmt.Println("Updated block chain.")
		ps.LocalChain.print()
	}
}

func (ps *PuzzlesState) handleBlockBroadcast(bb *BlockBroadcast, from *net.UDPAddr){
	added := ps.LocalChain.addBlock(bb.NewBlock)
	if(added){
		fmt.Println("Received new block and updated block chain.")
		ps.LocalChain.print()
		ps.broadcastBlock(bb.NewBlock, from)
	}
}

func (ps *PuzzlesState) handleJoining(joiner *net.UDPAddr){
	if(ps.LocalChain.LastBlock!=nil && ps.Joined){
		_,ok := ps.waiting[joiner.String()]
		if(!ok){
			ps.sendPuzzleProposal(joiner)
		}
	}
}

func (ps *PuzzlesState) sendPuzzleProposal(dest *net.UDPAddr){
	fmt.Println("Send puzzle proposal.")
	pp := &PuzzleProposal{ps.MyName,ps.LocalChain.nextNodeID(),time.Now(),ps.LocalChain.LastBlock.hash()}
	ps.waiting[dest.String()] = pp
	ps.send(&GossipPacket{PProposal: pp}, dest)
}

func (ps *PuzzlesState) send(msg *GossipPacket, dest_addr *net.UDPAddr){
	msg.NodeID = ps.MyID
    packetBytes, err1 := protobuf.Encode(msg)
    if(err1!=nil){
        fmt.Println(err1, " dest : ", dest_addr.String())
    }
    _,err2 := ps.conn.WriteToUDP(packetBytes,dest_addr)
    if err2 != nil {
        fmt.Println(err2, " dest : ", dest_addr.String())
    }
}