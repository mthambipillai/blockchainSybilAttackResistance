package main
import(
	"time"
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
	LocalChain	*BlockChain
}


func (ps *PuzzlesState) handlePuzzleProposal(pp *PuzzleProposal){

}

func (ps *PuzzlesState) handlePuzzleResponse(pp *PuzzleProposal){

}

func (ps *PuzzlesState) handleBlockBroadcast(pp *PuzzleProposal){

}