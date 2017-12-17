package main
import(
	"time"
	"net"
	"github.com/dedis/protobuf"
	"fmt"
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
}


func (ps *PuzzlesState) handlePuzzleProposal(pp *PuzzleProposal){

}

func (ps *PuzzlesState) handlePuzzleResponse(pp *PuzzleProposal){

}

func (ps *PuzzlesState) handleBlockBroadcast(pp *PuzzleProposal){

}

func (ps *PuzzlesState) sendPuzzleProposal(dest *net.UDPAddr){
	pp := &PuzzleProposal{ps.MyName,ps.LocalChain.nextNodeID(),time.Now(),ps.LocalChain.LastBlock.hash()}
	ps.send(&GossipPacket{PProposal: pp},dest)
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