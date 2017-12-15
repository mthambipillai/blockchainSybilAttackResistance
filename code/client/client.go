package main
//./client -UIPort=10000 -msg=Hello
import (
    "flag"
    "fmt"
    "net"
    "github.com/dedis/protobuf"
    "strconv"
    "encoding/hex"
    "strings"
)

type RumorMessage struct {
    Origin string
    ID int
    Text string
}

type PrivateMessage struct{
    Origin string
    ID int
    Text string
    Dest string
    HopLimit uint32
}

type PeerStatus struct {
    Identifier string
    NextID int
}

type StatusPacket struct {
    Want []PeerStatus
}

type GossipPacket struct {
    Rumor *RumorMessage
    Status *StatusPacket
    Message *PrivateMessage
    DRequest *DataRequest
    DReply *DataReply
    SRequest *SearchRequest
    SReply *SearchReply
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

type Gossiper struct {
    address *net.UDPAddr
    conn *net.UDPConn
    identifier string
}
 
func main() {
    UIPort := flag.Int("UIPort",10000,"port for gossiper communication")
    msgText := flag.String("msg","","message to send")
    dest := flag.String("Dest","","destination")
    file := flag.String("file","","file")
    request := flag.String("request","","file request")
    keywords := flag.String("keywords","","search keywords")
    budget := flag.Int("budget",2,"search budget")
    flag.Parse()
    var msg *GossipPacket
    if(*file!=""){
        if(*request==""){//sharing a file
            msg = &GossipPacket{Message : &PrivateMessage{"",-1,*file,"",0}}
        }else{//requesting a file
            hashBytes,err := hex.DecodeString(*request)
            if(err!=nil){
                panic(err)
            }
            msg = &GossipPacket{DRequest : &DataRequest{"",*dest,10,*file,hashBytes}}
        }
        
    }else{
        if(*keywords!=""){
            msg = &GossipPacket{SRequest : &SearchRequest{"",uint64(*budget),strings.Split(*keywords,",")}}
        }else{
            if(*dest==""){
                msg = &GossipPacket{Rumor : &RumorMessage{"",1,*msgText}}
            }else{
                msg = &GossipPacket{Message : &PrivateMessage{"",0,*msgText,*dest,10}}
            }
        }
    }
    
    myPort := 4000

    gossiperAddr,_ := net.ResolveUDPAddr("udp","127.0.0.1:"+strconv.Itoa(*UIPort))
    localAddr,_ := net.ResolveUDPAddr("udp", "127.0.0.1:"+strconv.Itoa(myPort))
 
    conn,_ := net.ListenUDP("udp", localAddr)
 
    defer conn.Close()
    send(msg,conn,gossiperAddr)
}

func send(msg *GossipPacket,conn *net.UDPConn, dest_addr *net.UDPAddr){
    fmt.Println("CLIENT")
    if(msg.Rumor!=nil){
        txt := msg.Rumor.Text
        fmt.Println("sending ",txt," to ",dest_addr)
    }
    if(msg.Message!=nil){
        if(msg.Message.ID==-1){
            fmt.Println("Sharing file ",msg.Message.Text)
        }else{
            d := msg.Message.Dest
            txt := msg.Message.Text
            fmt.Println("sending ",txt," to ",d)
        }  
    }
    packetBytes, err1 := protobuf.Encode(msg)
    if(err1!=nil){
        fmt.Println(err1)
    }
    _,err2 := conn.WriteToUDP(packetBytes,dest_addr)
    if err2 != nil {
        fmt.Println(err2)
    }
}