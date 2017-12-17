package main
//./gossiper -UIPort=10000 -gossipPort=127.0.0.1:5000 -name=nodeA -peers=127.0.0.1:5001 -rtimer=10
//./gossiper -UIPort=10001 -gossipPort=127.0.0.1:5001 -name=nodeB -peers=127.0.0.1:5002 -rtimer=10
//./gossiper -UIPort=10002 -gossipPort=127.0.0.1:5002 -name=nodeC -peers=127.0.0.1:5003 -rtimer=10
//./gossiper -UIPort=10003 -gossipPort=127.0.0.1:5003 -name=nodeD -peers=127.0.0.1:5002 -rtimer=10
import(
	"flag"
	"fmt"
	"net"
	"github.com/dedis/protobuf"
	"strconv"
	"strings"
	"time"
	"math/rand"
	"crypto/rsa"
)

func sendRumorToRandom(rumor *RumorMessage, state *State){
	peerAddr := state.pickRandomPeer().address
	state.sendRumorTo(rumor, peerAddr)
	state.waitForAck(rumor, peerAddr)
}

func receivesPrivateMessageFromPeer(pm *PrivateMessage, from *net.UDPAddr, state *State){
	if(pm.Dest==state.me.identifier){
		state.receivePrivateMessage(pm)
		printReceivePM(pm,from)
	}else{
		if(!state.noForward){
			success := state.forwardPrivateMessage(pm)
			if(success){
				printForwardPM(pm,from)
			}else{
				printFailForwardPM(pm,from)
			}
		}else{
			fmt.Println("Not forwarding private message "+pm.Text)
		}
	}
}

func receivesPrivateMessageFromClient(pm *PrivateMessage, state *State){
	pm.Origin = state.me.identifier
	if(pm.ID==-1){//hack to say this is a share file request
		fileName := pm.Text
		state.addFile(fileName)
		state.printFiles()
	}else{//this is really a private mesage
		state.sendPrivateMessageCLI(pm)
	}
}

func receivesDataRequestFromClient(dr *DataRequest, state *State){
	if(dr.Destination==""){
		go state.downloadGUI(dr.FileName)
	}else{
		go state.download(dr.FileName, dr.HashValue, dr.Destination)
	}
}

func receivesSearchRequestFromClient(sr *SearchRequest, state *State){
	state.startFileSearch(sr.Keywords, sr.Budget)
}

func receivesDataRequestFromPeer(dr *DataRequest, from *net.UDPAddr, state *State){
	if(dr.Destination==state.me.identifier){
		printReceiveDReq(dr, from)
		state.replyToDataRequest(dr)
	}else{
		printForwardDReq(dr, from)
		state.forwardDataRequest(dr)
	}
}

func receivesDataReplyFromPeer(dr *DataReply, from *net.UDPAddr, state *State){
	if(dr.Destination==state.me.identifier){
		printReceiveDRep(dr, from)
		state.handleDataReply(dr)
	}else{
		printForwardDRep(dr, from)
		state.forwardDataReply(dr)
	}
}

func receivesSearchRequestFromPeer(sr *SearchRequest, from *net.UDPAddr, state *State){
	if(sr.Origin!=state.me.identifier){//because the search might loop back to myself
		printReceiveSReq(sr)
		state.handleSearchRequest(sr, from)
	}
}

func receivesSearchReplyFromPeer(sr *SearchReply, from *net.UDPAddr, state *State){
	if(sr.Destination==state.me.identifier){
		printReceiveSRep(sr)
		state.handleSearchReply(sr)
	}else{
		printForwardSRep(sr,from)
		state.forwardSearchReply(sr)
	}
}

func receivesRumorFromPeer(rumor *RumorMessage, from *net.UDPAddr, state *State){
	state.addPeerFromLast(rumor)
	state.overwriteLast(rumor, from)
	if(rumor.Text==""){//this is a routing message
		updated := state.updateRoutingTable(rumor,from)
		if(updated){
			printRoutingTable(state)
		}
		peerAddr := state.pickRandomPeer().address
		state.sendRumorTo(rumor, peerAddr)
	}else{
		printPeers(state)
		printRumorPeer(rumor,from)
		if(!state.msgAlreadySeen(rumor)){
			updated := state.updateRoutingTable(rumor,from)
			if(updated){
				printRoutingTable(state)
			}
			updateVectorClock(state.vectorClock,rumor)
			state.addMsgSeen(rumor)

			if(!state.noForward){
				sendRumorToRandom(rumor, state)
				state.sendStatusTo(from)
			}
		}
	}
}

func receivesRumorFromClient(rumor *RumorMessage, state *State){
	rumor.Origin = state.me.identifier
	rumor.ID = state.clientNextID
	state.clientNextID = state.clientNextID+1
	printClient(rumor)

	updateVectorClock(state.vectorClock,rumor)
	state.addMsgSeen(rumor)
	if(!state.noForward){
		sendRumorToRandom(rumor, state)
	}
}

func receivesStatusFromPeer(status *StatusPacket, from *net.UDPAddr, state *State){
	printStatus(status,from)
	var rumor *RumorMessage
	if(state.isWaitingForAck(from)){
		rumor = state.getPendingMongering(from)
		state.terminateMongering(from)
	}
	notSeen, msgToSend := state.compareVectorClockWithOther(status)
	if(notSeen && !state.noForward){
		state.sendStatusTo(from)
	}
	if(msgToSend!=nil && !state.noForward){
		state.sendRumorTo(msgToSend, from)
	}
	if(!notSeen && msgToSend==nil && !state.noForward){
		printSync(from)
		printFlip(from)
		if(rand.Intn(2)==0 && rumor!=nil){
			sendRumorToRandom(rumor, state)
		}
	}
	//if we were waiting for a status from this peer -> stop timer, flip coin to continue mongering
}

func antiEntropy(state *State){
	ticker := time.NewTicker(time.Second*1).C
	go func(){
		for{
			select{
			case <- ticker:
				state.sendStatusTo(state.pickRandomPeer().address)
			}
		}
	}()
}

func removeSearches(state *State){
	ticker := time.NewTicker(time.Millisecond*500).C
	go func(){
		for{
			select{
			case <- ticker:
				state.searchReceived = make([]*SearchRequest,0)
			}
		}
	}()
}

func routingMessages(state *State, rtimer int){
	state.sendRoutingMessage()
	ticker := time.NewTicker(time.Second*time.Duration(rtimer)).C
	go func(){
		for{
			select{
			case <- ticker:
				state.sendRoutingMessage()
			}
		}
	}()
}

func main() {
	rand.Seed(time.Now().UnixNano())
	uiPort := flag.Int("UIPort",10000,"The port towards client")
	//guiAddress := flag.String("GUIIpPort","127.0.0.1:8080","the address for the web server")
	gossipIPPort := flag.String("gossipPort","127.0.0.1:5000","the port towards other gossipers")
	name := flag.String("name","node","name of gossiper")
	peers := flag.String("peers","127.0.0.1:5001","other gossipers")
	rtimer := flag.Int("rtimer",60,"period of sending routing mesages (seconds)")
	noforward := flag.Bool("noforward",false,"if set to true this peer will not forward any message.")
	genesis := flag.Bool("genesis",false,"if set to true this peer will start its own blockchain.")
	flag.Parse()

	bc := &BlockChain{}
	myID := uint64(1)
	if(*genesis){
		bc.initGenesis(myID,rsa.PublicKey{})
	}
	ps := &PuzzlesState{bc}
	srh := &SybilResistanceHandler{myID, bc, ps}
	ipPort := strings.Split(*gossipIPPort,":")
	ip :="127.0.0.1"
	if(len(ipPort)==2){
		ip = ipPort[0]
	}else{
		*gossipIPPort = "127.0.0.1:"+*gossipIPPort
	}

	myUIAddr, _:= net.ResolveUDPAddr("udp", ip+":"+strconv.Itoa(*uiPort))
	myUIConn, _:= net.ListenUDP("udp", myUIAddr)
	defer myUIConn.Close()
	me := newGossiper(*gossipIPPort,*name)
	myGossipConn, _:= net.ListenUDP("udp", me.address)
	me.conn = myGossipConn
	defer myGossipConn.Close()

	gossipers := parseOtherPeers(*peers)

	var peersStatus []PeerStatus
	vectorClock := &StatusPacket{Want : peersStatus}
	msgsSeen := make(map[RumorMessageKey]StoredRumor)
	pendingAcks := make(map[string]Mongering)
	lastSeqnSeen := make(map[string]int)
	nexthops := make(map[string]*net.UDPAddr)
	privateMsgs := make(map[string][]StoredPM)
	files := make(map[string]P2PFile)
	downloads := &Downloads{make(map[string]Download)}

	state := &State{me, gossipers, vectorClock, msgsSeen, pendingAcks, 1, lastSeqnSeen,
		nexthops, privateMsgs, *noforward, files, downloads, nil, false, make(chan string),
		nil,make([]*SearchRequest,0),0,srh}

	go webServer(state, ip+":"+strconv.Itoa(*uiPort-1000))
	go routingMessages(state, *rtimer)
	if(!state.noForward){
		antiEntropy(state)
	}
    go listenPeers(state)
    go removeSearches(state)
    listenClient(myUIConn, state)
}

func listenPeers(state *State){
    for {
    	packetBytes := make([]byte, 10000)
    	packet := &GossipPacket{}
        n,addr,_ := state.me.conn.ReadFromUDP(packetBytes)
        err := protobuf.Decode(packetBytes[:n], packet)
        if(err==nil){
        	isRumor := packet.Rumor!=nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isStatus := packet.Rumor==nil && packet.Status!=nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isMessage := packet.Rumor==nil && packet.Status==nil && packet.Message!=nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isDRequest := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest!=nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isDReply := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply!=nil && packet.SRequest==nil && packet.SReply==nil
        	isSRequest := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest!=nil && packet.SReply==nil
        	isSReply := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply!=nil
        	if(!state.alreadyKnowPeer(addr)){
        		state.addPeer(addr)
        	}
        	if(isStatus){
        		printPeers(state)
        		receivesStatusFromPeer(packet.Status, addr, state)
        	}else if(isRumor){
        		receivesRumorFromPeer(packet.Rumor, addr, state)
        	}else if(isMessage){
        		receivesPrivateMessageFromPeer(packet.Message, addr, state)
        	}else if(isDRequest){
        		receivesDataRequestFromPeer(packet.DRequest, addr, state)
    		}else if(isDReply){
    			receivesDataReplyFromPeer(packet.DReply, addr, state)
    		}else if(isSRequest){
    			receivesSearchRequestFromPeer(packet.SRequest, addr, state)
    		}else if(isSReply){
    			receivesSearchReplyFromPeer(packet.SReply, addr, state)
    		}else{
        		fmt.Println("Need to have either a rumor or a status or a private message.")
        	}
        }else{
        	fmt.Println(err)
        }
    }
}

func listenClient(conn *net.UDPConn, state *State){
    for {
    	packetBytes := make([]byte, 1024)
    	packet := &GossipPacket{}
        n,_,_ := conn.ReadFromUDP(packetBytes)
        err := protobuf.Decode(packetBytes[:n], packet)
        if(err==nil){
        	isRumor := packet.Rumor!=nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isStatus := packet.Rumor==nil && packet.Status!=nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isMessage := packet.Rumor==nil && packet.Status==nil && packet.Message!=nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isDRequest := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest!=nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply==nil
        	isDReply := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply!=nil && packet.SRequest==nil && packet.SReply==nil
        	isSRequest := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest!=nil && packet.SReply==nil
        	isSReply := packet.Rumor==nil && packet.Status==nil && packet.Message==nil && packet.DRequest==nil && packet.DReply==nil && packet.SRequest==nil && packet.SReply!=nil
        	if(isStatus){
        		fmt.Println("client not supposed to send status messages.")
        	}else if(isMessage){
        		receivesPrivateMessageFromClient(packet.Message, state)
        	}else if(isRumor){
        		printPeers(state)
    			receivesRumorFromClient(packet.Rumor, state)
        	}else if(isDRequest){
        		receivesDataRequestFromClient(packet.DRequest, state)
    		}else if(isDReply){
    			fmt.Println("client not supposed to send data replies.")
    		}else if(isSRequest){
    			receivesSearchRequestFromClient(packet.SRequest, state)
    		}else if(isSReply){
    			fmt.Println("client not supposed to send search replies.")
    		}else{
        		fmt.Println("Need to have only 1 non-nil field.")
        	}
        }else{
        	fmt.Println(err)
        }
    }
}