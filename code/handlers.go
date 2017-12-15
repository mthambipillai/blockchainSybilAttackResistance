package main

import(
	"net/http"
	"log"
	"github.com/gorilla/mux"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"strconv"
	"time"
)

func webServer(state *State, guiAddress string){
	
	r := mux.NewRouter()

	r.HandleFunc("/latestmessages", state.latestMessagesHandler).Methods("GET")
	r.HandleFunc("/latestprivatemessages", state.latestPrivateMessagesHandler).Methods("GET")
	r.HandleFunc("/nodes", state.nodesHandler).Methods("GET")
	r.HandleFunc("/routablepeers", state.routablePeersHandler).Methods("GET")
	r.HandleFunc("/node", state.addPeerHandler).Methods("POST")
	r.HandleFunc("/message", state.messageHandler).Methods("POST")
	r.HandleFunc("/privatemessage", state.privateMessageHandler).Methods("POST")
	r.HandleFunc("/id", state.saveNameHandler).Methods("POST")
	r.HandleFunc("/sharefile", state.shareFileHandler).Methods("POST")
	r.HandleFunc("/searchfiles", state.searchFilesHandler).Methods("POST")
	r.HandleFunc("/getsearch", state.getSearchHandler).Methods("GET")
	r.HandleFunc("/download", state.DownloadHandler).Methods("GET")
	
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("."))))

	srv := &http.Server{
		Handler:      r,
		Addr:         guiAddress,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func (state *State) latestMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	keys, ok := r.URL.Query()["from"]

	if !ok || len(keys) < 1 {
        log.Println("Url Param 'from' is missing")
        return
    }

    time := keys[0]
    timeInt, _ := strconv.Atoi(time)
    msgs := state.getMsgsFrom(int64(timeInt))
    var msgsStrings []string
    for _,m := range msgs{
    	msgsStrings = append(msgsStrings, m.toString())
    }
    jsonBytes,_ := json.Marshal(msgsStrings)
	w.Write(jsonBytes)
}

type PMsJSON struct{
	Pms []PMsPeer `json:"allprivatemessages"`
}

type PMsPeer struct{
	Peer string `json:"peer"`
	Pms []string `json:"privatemessages"`
}

func (state *State) latestPrivateMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	keys, ok := r.URL.Query()["from"]
	//peervals, ok2 := r.URL.Query()["peer"]

	if !ok || len(keys) < 1 {
        log.Println("Url Param 'from' is missing")
        return
    }
    /*if !ok2 || len(peervals) < 1 {
        log.Println("Url Param 'peer' is missing")
        return
    }*/

    time := keys[0]
    timeInt, _ := strconv.Atoi(time)
    //peer := peervals[0]
    var allPms []PMsPeer
    for peer,_ := range state.privateMsgs{
    	msgs := state.getPrivateMsgsFrom(int64(timeInt),peer)
	    var msgsStrings []string
	    for _,m := range msgs{
	    	msgsStrings = append(msgsStrings, m.toString())
	    }
	    pms := PMsPeer{peer,msgsStrings}
	    allPms = append(allPms, pms)
    }

    res := &PMsJSON{allPms}
    
    jsonBytes,_ := json.Marshal(res)
	w.Write(jsonBytes)
}

func (state *State) nodesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var peers []string
	for _,g := range state.gossipers{
		peers = append(peers, g.address.String())
	}
	jsonBytes,_ := json.Marshal(peers)
	w.Write(jsonBytes)
}

func (state *State) routablePeersHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var peers []string
	for peer,_ := range state.nextHops{
		peers = append(peers, peer)
	}
	jsonBytes,_ := json.Marshal(peers)
	w.Write(jsonBytes)
}

func (state *State) addPeerHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	s := strings.Split(string(body),"=")
	peer := s[1]
	for i:=0;i<len(peer);i++{
		if(string(peer[i])=="%"){
			peer = peer[:i]+":"+peer[i+3:]
		}
	}
	if(err==nil){
		state.addPeerFromString(peer)
		w.Write([]byte("Peer successfully added."))
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) messageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	s := strings.Split(string(body),"=")
	message := s[1]
	if(err==nil){
		rumor := &RumorMessage{"",1,message,nil,nil}
		receivesRumorFromClient(rumor,state)
		w.Write([]byte("Message successfully received."))
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) privateMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	if(err==nil){
		fields := strings.Split(string(body),"&")
		dest := strings.Split(fields[0],"=")[1]
		msg := strings.Split(fields[1],"=")[1]
		status := state.sendPrivateMessage(msg,dest)
		if(status){
			w.Write([]byte("Message successfully sent."))
		}else{
			w.Write([]byte("Message could not be sent."))
		}
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) saveNameHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	s := strings.Split(string(body),"=")
	newName := s[1]
	if(err==nil){
		state.changeOwnName(newName)
		w.Write([]byte("Name successfully changed."))
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) shareFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	s := strings.Split(string(body),"=")
	fileName := s[1]
	if(err==nil){
		state.addFile(fileName)
		state.printFiles()
		w.Write([]byte("File successfully shared."))
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) searchFilesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	body, err := ioutil.ReadAll(r.Body)
	s := strings.Split(string(body),"=")
	keywords := s[1]
	if(err==nil){
		state.searchToRetrieve = make([]string, 0)
		state.startFileSearch(strings.Split(keywords,"_"), 2)
		match := ""
		for match!="SEARCHISCOMPLETED"{
			match=<-state.currentSearch
			if(match!="SEARCHISCOMPLETED"){
				state.searchToRetrieve = append(state.searchToRetrieve, match)
				w.Write([]byte(match+"_"))
				if f, ok := w.(http.Flusher); ok {
			      f.Flush()
			    }
			}
		}
	}else{
		fmt.Println(err)
		w.Write([]byte("Decoding error."))
	}
	defer r.Body.Close()
}

func (state *State) getSearchHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	jsonBytes,_ := json.Marshal(state.searchToRetrieve)
	w.Write(jsonBytes)
}

func (state *State) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	keys, ok := r.URL.Query()["file"]
	if !ok || len(keys) < 1 {
        log.Println("Url Param 'from' is missing")
        return
    }
    fileName := keys[0]
    go state.downloadGUI(fileName)
}