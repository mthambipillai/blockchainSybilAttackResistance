package main

import(
	"fmt"
	"net"
	"github.com/dedis/protobuf"
	"strconv"
	"strings"
	"math/rand"
	"time"
)

type Mongering struct{
	Rumor *RumorMessage
	With *net.UDPAddr
	Ack *time.Timer
}

type PrivateMessage struct{
	Origin string
	ID int
    Text string
	Dest string
	HopLimit uint32
	LastIP *net.IP 
    LastPort *int
}

type RumorMessageKey struct{
	Origin string
	ID int
}

type RumorMessage struct {
    Origin string
    ID int
    Text string
    LastIP *net.IP 
    LastPort *int
}

type StoredRumor struct{
	Origin string
    ID int
    Text string
    TimeAdded int64
}

type StoredPM struct{
	Origin string
	ID int
    Text string
    TimeAdded int64
}

func (r *RumorMessage) getKey() RumorMessageKey {
	return RumorMessageKey{r.Origin,r.ID}
}

func (rumor *StoredRumor) toString() string{
	return "("+rumor.Origin+" id "+strconv.Itoa(rumor.ID)+") : "+rumor.Text
}

func (pm *StoredPM) toString() string{
	return pm.Origin+" : "+pm.Text
}

type PeerStatus struct {
    Identifier string
    NextID int
}

type StatusPacket struct {
    Want []PeerStatus
}

type GossipPacket struct {
	NodeID uint64
    Rumor *RumorMessage
    Status *StatusPacket
    Message *PrivateMessage
    DRequest *DataRequest
    DReply *DataReply
    SRequest *SearchRequest
    SReply *SearchReply
}

type Gossiper struct {
    address *net.UDPAddr
    conn *net.UDPConn
    identifier string
}

func newGossiper(address, identifier string) *Gossiper {
	udpAddr, _:= net.ResolveUDPAddr("udp", address)
	return &Gossiper{
		address: udpAddr,
		identifier: identifier,
	}
}

func parseOtherPeers(arg string) []*Gossiper{
	peers := strings.Split(arg,"_")
	var gossipers []*Gossiper
	for index,address := range peers {
		gossipers = append(gossipers,newGossiper(address,"P"+strconv.Itoa(index)))
	}
	return gossipers
}

type State struct{
	me *Gossiper
	gossipers []*Gossiper
	vectorClock *StatusPacket
	msgsSeen map[RumorMessageKey]StoredRumor
	pendingAcks map[string]Mongering
	clientNextID int
	lastSeqnSeen map[string]int //key is peer's name
	nextHops map[string]*net.UDPAddr //key is the destination name
	privateMsgs map[string][]StoredPM //key is the destination name
	noForward bool
	files map[string]P2PFile
	downloads *Downloads
	search map[string]*CurrentSearchResult //key is file name
	searchInProgress bool
	currentSearch chan string
	searchToRetrieve []string
	searchReceived []*SearchRequest
	budget uint64
	srh *SybilResistanceHandler
}

func (state *State) changeOwnName(newName string){
	state.me.identifier = newName
}

func send(msg *GossipPacket,conn *net.UDPConn, dest_addr *net.UDPAddr){
	if(msg.Rumor!=nil){
		if(msg.Rumor.Text!=""){
			printMongering(dest_addr)
		}else{
			printMongering2(dest_addr)
		}
	}

    packetBytes, err1 := protobuf.Encode(msg)
    if(err1!=nil){
        fmt.Println(err1, " dest : ", dest_addr.String())
    }
    _,err2 := conn.WriteToUDP(packetBytes,dest_addr)
    if err2 != nil {
        fmt.Println(err2, " dest : ", dest_addr.String())
    }
}

func (state *State) sendPrivateMessage(content string, dest string) bool{
	nexthop,ok := state.nextHops[dest]
	if(ok){
		pm := &PrivateMessage{state.me.identifier,0,content, dest, 10,nil,nil}
		send(&GossipPacket{Message : pm}, state.me.conn, nexthop)
		printSendPM(pm,nexthop)
		spm := StoredPM{state.me.identifier, pm.ID, pm.Text, time.Now().UnixNano() / int64(time.Millisecond)}
		state.privateMsgs[dest] = append(state.privateMsgs[dest],spm)
		return true
	}else{
		return false
	}
}
func (state *State) sendPrivateMessageCLI(pm *PrivateMessage) bool{
	nexthop,ok := state.nextHops[pm.Dest]
	if(ok){
		send(&GossipPacket{Message : pm}, state.me.conn, nexthop)
		printSendPM(pm,nexthop)
		spm := StoredPM{state.me.identifier, pm.ID, pm.Text, time.Now().UnixNano() / int64(time.Millisecond)}
		state.privateMsgs[pm.Dest] = append(state.privateMsgs[pm.Dest],spm)
		return true
	}else{
		return false
	}
}

func (state *State) receivePrivateMessage(pm *PrivateMessage){
	spm := StoredPM{pm.Origin, pm.ID, pm.Text, time.Now().UnixNano() / int64(time.Millisecond)}
	state.privateMsgs[pm.Origin] = append(state.privateMsgs[pm.Origin],spm)
}

func (state *State) forwardPrivateMessage(pm *PrivateMessage) bool{
	nexthop,ok := state.nextHops[pm.Dest]
	if(ok){
		pm.HopLimit = pm.HopLimit -1
		if(pm.HopLimit>0){
			send(&GossipPacket{Message : pm}, state.me.conn, nexthop)
			return true
		}else{
			return false
		}
	}else{
		return false
	}
}

func (state *State) alreadyKnowPeer(addr *net.UDPAddr) bool{
	for _,peer := range state.gossipers{
		if(peer.address.String()==addr.String()){
			return true
		}
	}
	return false
}

func (state *State) alreadyKnowPeerStr(addr string) bool{
	for _,peer := range state.gossipers{
		if(peer.address.String()==addr){
			return true
		}
	}
	return false
}

func (state *State) addPeer(addr *net.UDPAddr){
	newPeer := newGossiper(addr.String(),"P"+strconv.Itoa(len(state.gossipers)))
	state.gossipers = append(state.gossipers, newPeer)
}
func (state *State) addPeerFromString(addr string){
	address,_ := net.ResolveUDPAddr("udp", addr)
	state.addPeer(address)
}

func (state *State) pickRandomPeer() *Gossiper{
	if(len(state.gossipers)>0){
		return state.gossipers[rand.Intn(len(state.gossipers))]
	}else{
		return nil
	}	
}

func updateVectorClock(vectorClock *StatusPacket, rumor *RumorMessage){
	index := findPeerStatus(vectorClock, rumor.Origin)
	if(index==-1){
		if(rumor.ID==1){
			vectorClock.Want = append(vectorClock.Want,PeerStatus{rumor.Origin,2})
		}else{
			vectorClock.Want = append(vectorClock.Want,PeerStatus{rumor.Origin,1})
		}
	}else{
		status := vectorClock.Want[index]
		if(rumor.ID == status.NextID){
			vectorClock.Want[index].NextID = status.NextID + 1
		}
	}
}

func findPeerStatus(vectorClock *StatusPacket, origin string) int{
	for i,status := range vectorClock.Want{
		if(status.Identifier==origin){
			return i
		}
	}
	return -1
}

func (state *State) addMsgSeen(msg *RumorMessage) string{
	if(state.msgAlreadySeen(msg)){
		return "msg already seen"
	}
	state.msgsSeen[msg.getKey()] = StoredRumor{msg.Origin, msg.ID, msg.Text, time.Now().UnixNano() / int64(time.Millisecond)}
	return "msg successfully added"
}

func (state *State) compareVectorClockWithOther(vc *StatusPacket) (bool, *RumorMessage){
	someNotSeen := false
	var notSeenByOther RumorMessage
	b := false
	for _,s := range state.vectorClock.Want{
		index := findPeerStatus(vc,s.Identifier)
		if(index==-1){//the other has not seen any msg from this origin
			notSeenByOther = state.getMsg(s.Identifier,1)
			b = true
			break
		}else{
			n := vc.Want[index].NextID
			if(s.NextID < n){//the other has received more from this origin
				someNotSeen = true
			}else if(s.NextID > n){//the other has received less from this origin
				notSeenByOther = state.getMsg(s.Identifier, n)
				b = true
				break
			}
		}
	}
	for _,s := range vc.Want{
		index := findPeerStatus(state.vectorClock, s.Identifier)
		if(index==-1){//I have not seen any msg from this origin
			someNotSeen = true
		}
	}
	if(b){
		return someNotSeen, &notSeenByOther
	}else{
		return someNotSeen, nil
	}
}

func (state *State) getMsg(Origin string, ID int) RumorMessage{
	key := RumorMessageKey{Origin,ID}
	stored := state.msgsSeen[key]
	return RumorMessage{stored.Origin, stored.ID, stored.Text,nil,nil}
}

func (state *State) msgAlreadySeen(msg *RumorMessage) bool{
	_,ok := state.msgsSeen[msg.getKey()]
	return ok
}

func (state *State) getMsgsFrom(time int64)[]StoredRumor{
	var msgs []StoredRumor
	for _,msg := range state.msgsSeen{
		if(msg.TimeAdded>=time){
			msgs = append(msgs,msg)
		}
	}
	return msgs
}

func (state *State) getPrivateMsgsFrom(time int64, peer string)[]StoredPM{
	var msgs []StoredPM
	for _,msg := range state.privateMsgs[peer]{
		if(msg.TimeAdded>=time){
			msgs = append(msgs,msg)
		}
	}
	return msgs
}

func (state *State) sendRumorTo(msg *RumorMessage, dest *net.UDPAddr){
	send(&GossipPacket{Rumor : msg}, state.me.conn, dest)
}

func (state *State) sendStatusTo(dest *net.UDPAddr){
	send(&GossipPacket{Status : state.vectorClock}, state.me.conn, dest)
}

func (state *State) waitForAck(msg *RumorMessage, from *net.UDPAddr){
	t := time.NewTimer(time.Second*1)
	state.pendingAcks[from.String()] = Mongering{msg, from, t}
	go func() {
		<-t.C
		printFlip(from)
		if(rand.Intn(2)==0){
			sendRumorToRandom(msg, state)
		}
	}()
}

func (state *State) isWaitingForAck(from *net.UDPAddr) bool{
	_,ok := state.pendingAcks[from.String()]
	return ok
}

func (state *State) getPendingMongering(from *net.UDPAddr) *RumorMessage{
	m,ok := state.pendingAcks[from.String()]
	if(ok){
		return m.Rumor
	}else{
		return nil
	}
}

func (state *State) terminateMongering(with *net.UDPAddr){
	state.pendingAcks[with.String()].Ack.Stop()
	delete(state.pendingAcks, with.String())
}

func (state *State) updateRoutingTable(msg *RumorMessage, from *net.UDPAddr) bool{
	if(msg.Origin==state.me.identifier){
		return false
	}
	lastSeqN,ok := state.lastSeqnSeen[msg.Origin]
	if(!ok){
		state.lastSeqnSeen[msg.Origin] = msg.ID
		state.nextHops[msg.Origin] = from
		return true
	}else{
		if((msg.ID > lastSeqN) || (msg.ID == lastSeqN && isDirectRoute(msg))){
			state.lastSeqnSeen[msg.Origin] = msg.ID
			state.nextHops[msg.Origin] = from
			return true
		}else{
			return false
		}
	}
}

func (state *State) sendRoutingMessage(){
	rumor := &RumorMessage{state.me.identifier,-1,"",nil,nil}
	for _,peer := range state.gossipers{
		state.sendRumorTo(rumor,peer.address)
	}
}

func (state *State) overwriteLast(msg *RumorMessage, from *net.UDPAddr){
	addrStr := strings.Split(from.String(),":")
	ip := net.ParseIP(addrStr[0])
	msg.LastIP = &ip
	port,_ := strconv.Atoi(addrStr[1])
	msg.LastPort = &port
}

func (state *State) addPeerFromLast(msg *RumorMessage){
	ip := msg.LastIP
	port := msg.LastPort
	if(ip!=nil &&port!=nil){
		a := ip.String()+":"+strconv.Itoa(*port)
		if(!state.alreadyKnowPeerStr(a) && a!=state.me.address.String()){
			udpAddr, _:= net.ResolveUDPAddr("udp", a)
			state.addPeer(udpAddr)
		}
	}
}

func isDirectRoute(route *RumorMessage) bool{
	return route.Text=="" && route.LastIP==nil && route.LastPort==nil
}


func printStatus(status *StatusPacket, from *net.UDPAddr){
	s := "STATUS from "+from.String()
	for _,st := range status.Want{
		s = s + " origin "+st.Identifier+" nextID "+strconv.Itoa(st.NextID)
	}
	if(false) {
		fmt.Println(s)
	}
}

func printClient(rumor *RumorMessage){
	s := "CLIENT "+ rumor.Text+" "+rumor.Origin
	fmt.Println(s)
}

func printRumorPeer(rumor *RumorMessage, from *net.UDPAddr){
	s := "RUMOR origin "+rumor.Origin+" from "+from.String()+" ID "+strconv.Itoa(rumor.ID)+" contents "+rumor.Text
	fmt.Println(s)
}

func printForwardPM(pm *PrivateMessage, from *net.UDPAddr){
	s := "FORWARDING origin "+pm.Origin+" from "+from.String()+" to "+pm.Dest+" contents "+pm.Text
	fmt.Println(s)
}

func printFailForwardPM(pm *PrivateMessage, from *net.UDPAddr){
	s := "FAILED TO FORWARD origin "+pm.Origin+" from "+from.String()+" to "+pm.Dest+" contents "+pm.Text
	fmt.Println(s)
}

func printReceivePM(pm *PrivateMessage, from *net.UDPAddr){
	s := "PRIVATE: "+pm.Origin+":"+strconv.Itoa(int(pm.HopLimit))+":"+pm.Text
	fmt.Println(s)
}

func printSendPM(pm *PrivateMessage, to *net.UDPAddr){
	s := "SENDING dest "+pm.Dest+" to "+to.String()+" contents "+pm.Text
	fmt.Println(s)
}

func printMongering(dest_addr *net.UDPAddr){
	s := "MONGERING TEXT with "+dest_addr.String()
	if(false) {
		fmt.Println(s)
	}
}

func printMongering2(dest_addr *net.UDPAddr){
	s := "MONGERING ROUTE with "+dest_addr.String()
	if(false) {
		fmt.Println(s)
	}
}

func printFlip(to *net.UDPAddr){
	s := "FLIPPED COIN sending rumor to "+to.String()
	if(false) {
		fmt.Println(s)
	}
}

func printSync(with *net.UDPAddr){
	s := "IN SYNC WITH "+with.String()
	if(false) {
		fmt.Println(s)
	}
}

func printPeers(state *State){
	s := ""
	for _,p := range state.gossipers{
		s = s+p.address.String()+","
	}
	s = s[0:len(s)-1]
	if(false) {
		fmt.Println(s)
	}
}

func printRoutingTable(state *State){
	if(false){
		fmt.Println("New routing table:(Dest:Nexthop)")
		for dest,nexthop := range state.nextHops{
			s := "DSDV "+dest+":"+nexthop.String()
			fmt.Println(s)
		}
	}
}