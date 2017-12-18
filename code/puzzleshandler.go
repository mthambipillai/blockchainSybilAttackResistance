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
}


func (ps *PuzzlesState) handlePuzzleProposal(pp *PuzzleProposal, from *net.UDPAddr){
	fmt.Println("start mining")
	b := mineBlock(pp.NodeID, pp.Timestamp, rsa.PublicKey{}, pp.PreviousHash)
	fmt.Println("done mining")
	pr := &PuzzleResponse{ps.MyName, pp.Origin, b}
	ps.send(&GossipPacket{PResponse: pr}, from)
}

func (ps *PuzzlesState) handlePuzzleResponse(pr *PuzzleResponse, from *net.UDPAddr){
	fmt.Println("got pr")
}

func (ps *PuzzlesState) handleBlockBroadcast(bb *BlockBroadcast){

}

func (ps *PuzzlesState) sendPuzzleProposal(dest *net.UDPAddr){
	pp := &PuzzleProposal{ps.MyName,ps.LocalChain.nextNodeID(),time.Now(),ps.LocalChain.LastBlock.hash()}
	ps.send(&GossipPacket{PProposal: pp}, dest)
}

func (ps *PuzzlesState) send(msg *GossipPacket, dest_addr *net.UDPAddr){
	fmt.Println("send ps to ",*dest_addr)
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