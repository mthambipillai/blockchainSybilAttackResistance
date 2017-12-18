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

type PuzzlesState struct{
	MyID		uint64
	MyName		string
	LocalChain	*BlockChain
	conn		*net.UDPConn
	waiting		[]string
}


func (ps *PuzzlesState) handlePuzzleProposal(pp *PuzzleProposal, from *net.UDPAddr){
	fmt.Println("Received puzzle proposal. Start mining.")
	b := mineBlock(pp.NodeID, pp.Timestamp, rsa.PublicKey{}, pp.PreviousHash)
	fmt.Println("Done mining. Send puzzle response.")
	pr := &PuzzleResponse{ps.MyName, pp.Origin, b}
	ps.send(&GossipPacket{PResponse: pr}, from)
}

func (ps *PuzzlesState) handlePuzzleResponse(pr *PuzzleResponse, from *net.UDPAddr){
	fmt.Println("Received puzzle response.")
}

func (ps *PuzzlesState) handleBlockBroadcast(bb *BlockBroadcast){

}

func (ps *PuzzlesState) handleJoining(joiner *net.UDPAddr){
	if(ps.LocalChain.LastBlock!=nil){
		for _,j := range(ps.waiting){
			if(j==joiner.String()){
				return
			}
		}
		ps.sendPuzzleProposal(joiner)
		ps.waiting = append(ps.waiting, joiner.String())
	}
}

func (ps *PuzzlesState) sendPuzzleProposal(dest *net.UDPAddr){
	fmt.Println("Send puzzle proposal.")
	pp := &PuzzleProposal{ps.MyName,ps.LocalChain.nextNodeID(),time.Now(),ps.LocalChain.LastBlock.hash()}
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