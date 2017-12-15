package main
import(
	"fmt"
	"os"
	"log"
	"crypto/sha256"
	"net"
	"encoding/hex"
	"time"
	"strings"
	"strconv"
)

type CurrentSearchResult struct{
	FileName string
	MetafileHash []byte
	ChunksLocations map[uint64][]string //key is chunk's index, value is peer's name
}

type SearchRequest struct {
	Origin string
	Budget uint64
	Keywords []string
}

type SearchReply struct {
	Origin string
	Destination string
	HopLimit uint32
	Results []*SearchResult
}
type SearchResult struct {
	FileName string
	MetafileHash []byte
	ChunkMap []uint64
}

type P2PFile struct{
	FileName string
	Size int
	Metafile []byte
	MetaHash []byte
	Chunks map[string][]byte //key is hash, value is chunk
}

type DataRequest struct {
	Origin string
	Destination string
	HopLimit uint32
	FileName string
	HashValue []byte
}

type DataReply struct {
	Origin string
	Destination string
	HopLimit uint32
	FileName string
	HashValue []byte
	Data []byte
}

type Downloads struct{
	Pendings map[string]Download //key is peer
}

type Download struct{
	Hash string
	Timeout *time.Timer
	Data chan []byte
}

func (state *State) addFile(name string){
	file, err := os.Open(name)
	if err != nil {
		log.Fatal(err)
	}

	data := make([]byte, 2000000)//max size is 2MB
	chunkSize := 8000
	count, err := file.Read(data)
	if err != nil {
		log.Fatal(err)
	}
	var metafile []byte
	chunks := make(map[string][]byte)
	for i := 0; i<count; i = i + chunkSize{
		end := i+chunkSize
		if(end>count){
			end = count
		}
		chunk := data[i:end]
		h := hash(chunk)
		metafile = append(metafile,h...)
		chunks[hex.EncodeToString(h)] = chunk
	}
	p2pfile := P2PFile{name,count,metafile,hash(metafile),chunks}
	state.files[name] = p2pfile
	printAddFile(&p2pfile)
}



func (state *State) printFiles(){
	fmt.Println("Files :")
	for _,file := range state.files{
		fmt.Println(file.FileName)
	}
}


func (state *State) download(filename string, metahash []byte, from string){
	s := "DOWNLOADING metafile of "+filename+" from "+from
	fmt.Println(s)
	dataCh := make(chan []byte,10000)
	state.sendDataRequest(from, filename, metahash, dataCh)
	metafile := <-dataCh
	state.storeData(filename,hex.EncodeToString(metahash),metafile)
	hashes := splitMetafile(metafile)
	for _,h := range(hashes){
		hashBytes,err := hex.DecodeString(h)
		if(err!=nil){
			panic(err)
		}
		s = "DOWNLOADING "+filename+" chunk "+strconv.Itoa(state.chunkIndex(hashes,h))+" from "+from
		fmt.Println(s)
		state.sendDataRequest(from, filename, hashBytes, dataCh)
		chunk := <-dataCh
		state.storeData(filename, h, chunk)
	}
	storeFile(state.files[filename])
	s = "RECONSTRUCTED file "+filename
	fmt.Println(s)
}

func (state *State) downloadGUI(filename string){
	searchResults,ok := state.search[filename]
	if(!ok){
		fmt.Println("cannot download ",filename)
		return
	}
	dataCh := make(chan []byte,10000)
	var possiblePeers []string
	for _,peers := range(searchResults.ChunksLocations){
		for _,p := range(peers){
			if(!arrayContains(possiblePeers,p)){
				possiblePeers = append(possiblePeers, p)
			}
		}
	}
	s := "DOWNLOADING metafile of "+filename+" from "+possiblePeers[0]
	fmt.Println(s)
	state.sendDataRequest(possiblePeers[0], filename, searchResults.MetafileHash, dataCh)
	metafile := <-dataCh
	state.storeData(filename,hex.EncodeToString(searchResults.MetafileHash),metafile)
	hashes := splitMetafile(metafile)
	for index,h := range(hashes){
		hashBytes,err := hex.DecodeString(h)
		if(err!=nil){
			panic(err)
		}
		peer := searchResults.ChunksLocations[uint64(index)][0]
		s = "DOWNLOADING "+filename+" chunk "+strconv.Itoa(state.chunkIndex(hashes,h))+" from "+peer
		fmt.Println(s)
		state.sendDataRequest(peer, filename, hashBytes, dataCh)
		chunk := <-dataCh
		state.storeData(filename, h, chunk)
	}
	storeFile(state.files[filename])
	s = "RECONSTRUCTED file "+filename
	fmt.Println(s)
}

func (state *State) sendDataRequest(dest string, filename string, hash []byte, data chan []byte){
	nexthop,ok := state.nextHops[dest]
	if(ok){
		dr := &DataRequest{state.me.identifier, dest, 10, filename, hash}
		printSendDReq(dr)
		send(&GossipPacket{DRequest: dr},state.me.conn, nexthop)
		t := time.NewTimer(time.Second*5)
		state.setPending(dest,hex.EncodeToString(hash), t, data)
		go func() {
			<-t.C
			state.sendDataRequest(dest,filename,hash, data)
		}()
	}
}

func (state *State) forwardDataRequest(dr *DataRequest){
	nexthop,ok := state.nextHops[dr.Destination]
	if(ok){
		send(&GossipPacket{DRequest: dr},state.me.conn, nexthop)
	}
}

func (state *State) forwardDataReply(dr *DataReply){
	nexthop,ok := state.nextHops[dr.Destination]
	if(ok){
		send(&GossipPacket{DReply: dr},state.me.conn, nexthop)
	}
}

func (state *State) replyToDataRequest(dr *DataRequest){
	data := state.getData(dr.FileName, hex.EncodeToString(dr.HashValue))
	if(data!=nil){
		nexthop,ok := state.nextHops[dr.Origin]
		if(ok){
			dReply := &DataReply{state.me.identifier, dr.Origin, 10, dr.FileName, dr.HashValue, data}
			printSendDRep(dReply)
			send(&GossipPacket{DReply: dReply},state.me.conn, nexthop)
		}	
	}
}

func (state *State) handleDataReply(dr *DataReply){
	waitingHash := state.getPending(dr.Origin)
	if(waitingHash!=""){
		hashesMatch := (waitingHash==hex.EncodeToString(hash(dr.Data)))
		if(hashesMatch){
			state.stopTimerAndSendData(dr.Origin, dr.Data)
		}
	}
}

func (state *State) getPending(peer string)string{
	d, ok := state.downloads.Pendings[peer]
	if(ok){
		return d.Hash
	}else{
		return ""
	}
}

func (state *State) stopTimerAndSendData(peer string, data []byte){
	d, ok := state.downloads.Pendings[peer]
	if(ok){
		d.Timeout.Stop()
		d.Data <- data
	}
}

func (state *State) setPending(peer string, hash string, t *time.Timer, data chan []byte){
	state.downloads.Pendings[peer] = Download{hash,t,data}
}

func (state *State) getData(filename string, hash string)[]byte{
	file,ok := state.files[filename]
	if(ok){
		if(hex.EncodeToString(file.MetaHash)==hash){
			return file.Metafile
		}else{
			chunk,ok2 := file.Chunks[hash]
			if(ok2){
				return chunk
			}else{
				return nil
			}
		}
	}else{
		return nil
	}
}

func (state *State) storeData(filename string, hash string, data []byte){
	file,ok := state.files[filename]
	if(ok){
		file.Chunks[hash] = dataDeepCopy(data)
		file.Size = file.Size + len(data)
	}else{
		hashBytes,_ := hex.DecodeString(hash)
		state.files[filename] = P2PFile{filename,0,dataDeepCopy(data),hashBytes,make(map[string][]byte)}
	}
}

func (state *State) startFileSearch(keywords []string, budget uint64){
	if(state.searchInProgress){
		fmt.Println("cannot start search. Another search is in progress.")
		return
	}
	state.budget=budget
	state.search = make(map[string]*CurrentSearchResult)
	fmt.Println("search ",keywords," with budget ",budget)
	maxBudget := uint64(32)
	ticker := time.NewTicker(time.Second*1)
	t := ticker.C
	go func(){
		for{
			select{
			case <- t:
				nbMatches := len(state.search)
				if(budget <= maxBudget && nbMatches <2){
					budgets := state.splitBudget(budget, state.gossipers)
					for i,b := range(budgets){
						sr := &SearchRequest{state.me.identifier, b, keywords}
						send(&GossipPacket{SRequest: sr},state.me.conn, state.gossipers[i].address)
					}
					budget = budget*uint64(2)
					state.budget = budget
				}else{
					ticker.Stop()
					fmt.Println("SEARCH FINISHED")
					if(len(state.search)>0){
						fmt.Println("search finished with results:")
					}else{
						fmt.Println("search finished with empty results.")
					}
					for f,res := range(state.search){
						fmt.Println("file "+f+" : ")
						for index,peers := range(res.ChunksLocations){
							fmt.Println("chunk ",index," at peer(s): "+strings.Join(peers,","))
						}
					}
					state.searchInProgress=false
					state.currentSearch <- "SEARCHISCOMPLETED"
				}
			}
		}
	}()
}

func (state *State) splitBudget(budget uint64, peers []*Gossiper)[]uint64{
	var res []uint64
	if(len(peers)>=int(budget)){
		for range(make([]int,int(budget))){
			res = append(res,uint64(1))
		}
	}else{
		baseBudget := uint64(budget/uint64(len(peers)))
		for range(peers){
			res = append(res,baseBudget)
		}
		remaining := budget - uint64(len(peers))*baseBudget
		for i:=0;i<int(remaining);i=i+1{
			res[i] = res[i]+1
		}
	}
	return res
}

func (state *State) findMatches(keywords []string)[]*SearchResult{
	var res []*SearchResult
	for filename,file := range(state.files){
		for _,k := range(keywords){
			if(strings.Contains(filename,k)){
				mf := splitMetafile(file.Metafile)
				var chunkMap []uint64
				for hash,_ := range(file.Chunks){
					index := state.chunkIndex(mf,hash)
					if(index!=-1){
						chunkMap = append(chunkMap,uint64(index))
					}
				}
				sr := &SearchResult{filename, file.MetaHash, chunkMap}
				res = append(res, sr)
				break
			}
		}
	}
	return res
}

func (state *State) chunkIndex(metafile []string, hash string)int{
	for i,h := range(metafile){
		if(h==hash){
			return i
		}
	}
	return -1
}

func (state *State) handleSearchRequest(sr *SearchRequest, from *net.UDPAddr){
	if(state.isDuplicate(sr)){
		return
	}
	state.searchReceived = append(state.searchReceived, sr)
	if(sr.Budget>1){
		state.forwardSearchRequest(sr, from)
	}
	results := state.findMatches(sr.Keywords)
	if(results!=nil){
		nexthop,ok := state.nextHops[sr.Origin]
		if(ok){
			reply := &SearchReply{state.me.identifier, sr.Origin, 10, results}
			printSendSRep(reply)
			send(&GossipPacket{SReply: reply},state.me.conn, nexthop)
		}
	}
}

func (state *State) isDuplicate(sr *SearchRequest)bool{
	for _,srStored := range state.searchReceived{
		if(strings.Join(srStored.Keywords,",")==strings.Join(sr.Keywords,",") && srStored.Origin==sr.Origin){
			return true
		}
	}
	return false
}

func (state *State) handleSearchReply(sr *SearchReply){
	for _,res := range(sr.Results){
		state.addSearchResult(res,sr.Origin)
	}
}

func (state *State) addSearchResult(sr *SearchResult, from string){
	ch := make([]string,0)
	for _,chIndex := range(sr.ChunkMap){
		ch = append(ch,strconv.Itoa(int(chIndex)))
	}
	s := "FOUND match "+sr.FileName+" at "+from+" budget="+strconv.Itoa(int(state.budget))+
	" metafile="+hex.EncodeToString(sr.MetafileHash)+" chunks ="+strings.Join(ch,",")
	fmt.Println(s)
	currentRes,ok := state.search[sr.FileName]
	if(ok){
		if(hex.EncodeToString(sr.MetafileHash)==hex.EncodeToString(currentRes.MetafileHash)){
			chunksLoc := currentRes.ChunksLocations
			for _,index := range(sr.ChunkMap){
				if(!arrayContains(chunksLoc[index],from)){
					chunksLoc[index] = append(chunksLoc[index],from)
				}
			}
		}
	}else{
		chunksLoc := make(map[uint64][]string)
		for _,index := range(sr.ChunkMap){
			chunksLoc[index] = []string{from}
		}
		res := &CurrentSearchResult{sr.FileName, sr.MetafileHash, chunksLoc}
		state.search[sr.FileName] = res
		state.currentSearch <- sr.FileName
	}
}

func arrayContains(array []string, str string)bool{
	for _,s := range(array){
		if(s==str){
			return true
		}
	}
	return false
}

func (state *State) forwardSearchRequest(sr *SearchRequest, from *net.UDPAddr){
	budget := sr.Budget -1
	var otherPeers []*Gossiper
	for _,g := range(state.gossipers){
		if(g.address.String()!=from.String()){
			otherPeers = append(otherPeers, g)
		}
	}
	budgets := state.splitBudget(budget, otherPeers)
	for i,b := range(budgets){
		nsr := &SearchRequest{sr.Origin, b, sr.Keywords}
		printSendSReq(nsr)
		send(&GossipPacket{SRequest: nsr},state.me.conn, otherPeers[i].address)
	}
}

func (state *State) forwardSearchReply(sr *SearchReply){
	nexthop,ok := state.nextHops[sr.Destination]
	if(ok){
		send(&GossipPacket{SReply: sr},state.me.conn, nexthop)
	}
}

func hash(chunk []byte)[]byte{
	hasher := sha256.New()
	hasher.Write(chunk)
	return hasher.Sum(nil)
}

func splitMetafile(metafile []byte)[]string{
	var hashes []string
	for i:=0;i<len(metafile);i+=32{
		hashes = append(hashes,hex.EncodeToString(metafile[i:(i+32)]))
	}
	return hashes
}

func storeFile(file P2PFile){
	var data []byte
	for _,hash := range(splitMetafile(file.Metafile)){
		data = append(data,file.Chunks[hash]...)
	}
	f, err := os.Create("_Downloads/"+file.FileName)
	defer f.Close()
	if(err!=nil){
		fmt.Println(err)
	}else{
		_, err := f.Write(data)
		if(err!=nil){
			fmt.Println(err)
		}
	}
}

func dataDeepCopy(data []byte)[]byte{
	var res []byte
	for _,b := range(data){
		res = append(res,b)
	}
	return res
}

func printForwardDReq(dr *DataRequest, from *net.UDPAddr){
	s := "FORWARDING Data Request origin "+dr.Origin+" from "+from.String()+" to "+dr.Destination+" for file "+dr.FileName
	fmt.Println(s)
}

func printForwardDRep(dr *DataReply, from *net.UDPAddr){
	s := "FORWARDING Data Reply origin "+dr.Origin+" from "+from.String()+" to "+dr.Destination+" for file "+dr.FileName
	fmt.Println(s)
}

func printForwardSRep(sr *SearchReply, from *net.UDPAddr){
	s := "FORWARDING Search Reply origin "+sr.Origin+" from "+from.String()+" to "+sr.Destination
	fmt.Println(s)
}

func printReceiveDReq(dr *DataRequest, from *net.UDPAddr){
	s := "RECEIVING Data Request origin "+dr.Origin+" from "+from.String()+" for file "+dr.FileName
	fmt.Println(s)
}

func printReceiveDRep(dr *DataReply, from *net.UDPAddr){
	s := "RECEIVING Data Reply origin "+dr.Origin+" from "+from.String()+" for file "+dr.FileName
	fmt.Println(s)
}

func printSendDReq(dr *DataRequest){
	s := "SENDING Data Request to "+dr.Destination+" for file "+dr.FileName
	fmt.Println(s)
}

func printSendDRep(dr *DataReply){
	s := "SENDING Data Reply to "+dr.Destination+" for file "+dr.FileName
	fmt.Println(s)
}

func printSendSRep(sr *SearchReply){
	s := "SENDING Search Reply to "+sr.Destination+" with results:"
	fmt.Println(s)
	for _,res := range(sr.Results){
		fmt.Println(res.FileName)
	}
}

func printSendSReq(sr *SearchRequest){
	s := "SENDING Search Request for keywords "+strings.Join(sr.Keywords,",")
	fmt.Println(s)
}

func printReceiveSRep(sr *SearchReply){
	s := "RECEIVING Search Reply from "+sr.Origin+" with results:"
	fmt.Println(s)
	for _,res := range(sr.Results){
		fmt.Println(res.FileName)
	}
}

func printReceiveSReq(sr *SearchRequest){
	s := "RECEIVING Search Request from "+sr.Origin+" for keywords "+strings.Join(sr.Keywords,",")
	fmt.Println(s)
}

func printAddFile(file *P2PFile){
	s := "ADDED FILE "+file.FileName+" with meta hash "+hex.EncodeToString(file.MetaHash)
	fmt.Println(s)
}