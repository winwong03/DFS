package main

import (
	"encoding/json"
	"errors"
	//"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"time"

	"./shared"
)

// A Chunk is the unit of reading/writing in DFS.
type Chunk [32]byte

// Represents a type of file access.
type FileMode int

const (
	// Read mode.
	READ FileMode = iota

	// Read/Write mode.
	WRITE

	// Disconnected read mode.
	DREAD
)

func main() {
	// check that command line args present
	args := os.Args[1:]
	if len(args) != 1 {
		log.Fatal("Not enough arguments in call")
	}

	// Register RPC handler
	dfs := NewDFSServerInstance()
	dfs.Count = 1
	dfs.ConnectedClients = make(map[string]string)
	dfs.ClientInfo = make(map[string]*shared.ClientMetadata)
	dfs.Access = make(map[string]string)
	dfs.ClientFiles = make(map[string][]*shared.FileMetadata)
	dfs.FileVersions = make(map[string][256]int)
	dfs.Files = make(map[string][]string)
	dfs.Heartbeat = make(map[string]time.Time)
	dfs.HeartbeatDisconnected = make(map[string]bool)
	dfs.Originals = make(map[string][]string)
	dfs.ClientsWriting = make(map[string]string)
	dfs.Clients = make(map[string]*rpc.Client)

	addr := os.Args[1]
	// Create heartbeat server in another goroutine
	serverAddr, _ := net.ResolveUDPAddr("udp", addr)
	heartbeat, _ := net.ListenUDP("udp", serverAddr)
	dfs.HeartbeatServer = heartbeat
	go UDPHeartbeatListener(heartbeat, dfs)

	// Get address of client-incoming ip:port
	//fmt.Printf("\nListening for clients at:%v...\n", addr)
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		//fmt.Println("Problem resolvin addr:%s\n", addr)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Fatal("Listening error occurred: ", err)
	}
	server := rpc.NewServer()
	server.Register(dfs)

	// Serve in another goroutine
	for {
		// Accept connections and block until listener receives non-nil error
		conn, _ := listener.Accept()
		go server.ServeConn(conn)
	}
}

// Listen for heartbeats
func UDPHeartbeatListener(listener *net.UDPConn, dfs *DFSServerInstance) {
	buffer := make([]byte, 1024)
	for {
		size, _ := listener.Read(buffer[0:])
		if len(buffer) > 0 {
			//fmt.Printf("Buffer contents:%s, size:%d\n", buffer, len(buffer))
			ReportHeartbeat(buffer[:size], dfs)
		}
	}
}

// Reports heartbeats and updates server
func ReportHeartbeat(buffer []byte, dfs *DFSServerInstance) {
	var heartbeat shared.Heartbeat

	_ = json.Unmarshal(buffer, &heartbeat)

	// Get previous heartbeat time
	lastHeartbeat := dfs.Heartbeat[heartbeat.LocalPath]

	// Save new heartbeat time
	newHeartbeat := heartbeat.TimeSent
	dfs.Heartbeat[heartbeat.LocalPath] = newHeartbeat

	// Get time difference
	duration := newHeartbeat.Sub(lastHeartbeat)
	if duration > 2 {
		//fmt.Printf("DISCONNECTED CLIENT %v\n", heartbeat.LocalPath)
		dfs.ConnectedClients[heartbeat.LocalPath] = "Disconnected"
		dfs.HeartbeatDisconnected[heartbeat.LocalPath] = true
		// Remove write access
		filename, exists := dfs.ClientsWriting[heartbeat.LocalPath]
		if exists {
			delete(dfs.ClientsWriting, heartbeat.LocalPath)
			delete(dfs.Access, filename)
		}
	}
}

type DFSServerInstance struct {
	// Metadata
	Count                 int                               // ID to give to servers
	Originals             map[string][]string               // map original to new given localPath
	ConnectedClients      map[string]string                 // record clients connected to server ie: [Client1:Connected]
	ClientInfo            map[string]*shared.ClientMetadata // Information about clients (addr, localpath, files) ie: [/tmp/dev: stuff about client]
	Access                map[string]string                 // Which clients reading/writing ie:
	ClientFiles           map[string][]*shared.FileMetadata // what files do clients have [/tmp/dev/2: List of Files]
	FileVersions          map[string][256]int               // Files with list of their versions
	Files                 map[string][]string               // What files are in network and what client has them
	Client                *rpc.Client                       // Client to send rpc to other Clients
	Clients               map[string]*rpc.Client
	Heartbeat             map[string]time.Time // Client's latest heartbeat
	HeartbeatDisconnected map[string]bool      //Records whether client disconnected
	HeartbeatServer       *net.UDPConn
	ClientsWriting        map[string]string
}

func NewDFSServerInstance() *DFSServerInstance {
	return &DFSServerInstance{}
}

// Connect to client RPC server
// Add initial client info to server metadata
func (d *DFSServerInstance) Mount(args *shared.Args, reply *shared.Reply) (err error) {
	connected := true

	// Connect to client
	//fmt.Printf("Address that rpc server is sending to: %s\n", args.Addr) //delete
	conn, err := rpc.Dial("tcp", args.Addr)
	if err != nil {
		connected = false
	}

	//d.Client = conn
	d.Clients[args.LocalPath] = conn

	// TODO find an empty disconnected Client slot number
	// Good to have but dw about this for now - max num of clients in assgmt is 16

	// Ignore this new path thing for now?
	//newLocalPath := fmt.Sprintf("%s%d/", args.LocalPath, d.Count)
	client := &shared.ClientMetadata{
		ID:        d.Count,
		Addr:      args.Addr,
		LocalPath: args.LocalPath,
	}

	// Remove old data if client has mounted before
	delete(d.Access, args.LocalPath)
	delete(d.Heartbeat, args.LocalPath)
	delete(d.ClientsWriting, args.LocalPath)
	delete(d.HeartbeatDisconnected, args.LocalPath)

	if !connected {
		d.ConnectedClients[client.LocalPath] = "Disconnected"
	} else {
		d.ConnectedClients[client.LocalPath] = "Connected"
	}
	d.ClientInfo[client.LocalPath] = client
	reply.ID = d.Count
	reply.LocalPath = client.LocalPath
	//fmt.Printf("Client %s is connected.\n", client.LocalPath)
	d.Count++

	return nil
}

func (d *DFSServerInstance) RemoveAccess(args *shared.Args, reply *shared.Reply) (err error) {
	localPath, _ := d.Access[args.Filename]
	delete(d.Access, args.Filename)
	delete(d.ClientsWriting, localPath)
	return nil
}

// If file is already being written to, return false
// Else return true and update server to know it's being written to
func (d *DFSServerInstance) Writeable(args *shared.Args, reply *shared.Reply) (err error) {
	writer := d.Access[args.Filename]
	if len(d.Access[args.Filename]) > 0 && writer != args.LocalPath {
		//fmt.Printf("File %f is not writable, being written to by %s\n", args.Filename, writer)
		reply.Writeable = false
	} else {
		reply.Writeable = true
		//Tell server we're writing
		if args.Mode == int(WRITE) {
			d.Access[args.Filename] = args.LocalPath
			d.ClientsWriting[args.LocalPath] = args.Filename
		}
	}
	return nil
}

// Update chunk to some version
func (d *DFSServerInstance) UpdateChunkVersion(args *shared.Args, reply *shared.Reply) (err error) {
	clientInfo := d.ClientInfo[args.LocalPath]
	files := clientInfo.Files
	var file *shared.FileMetadata
	var index int
	for i, f := range files {
		if f.Name == args.Filename {
			file = f
			index = i
		}
	}
	ver := file.Versions
	version := ver[int(args.Chunknum)]
	ver[int(args.Chunknum)] = version + 1

	newFile := &shared.FileMetadata{
		Name:     file.Name,
		Versions: ver,
	}
	newFiles := files
	newFiles[index] = newFile
	newClient := &shared.ClientMetadata{
		ID:        clientInfo.ID,
		Addr:      clientInfo.Addr,
		LocalPath: clientInfo.LocalPath,
		Files:     newFiles,
	}
	d.ClientInfo[args.LocalPath] = newClient
	d.ClientFiles[args.LocalPath] = newFiles
	newVersions := d.FileVersions[args.Filename]
	newVersions[int(args.Chunknum)] = version + 1
	d.FileVersions[args.Filename] = newVersions

	reply.Version = version + 1
	return nil
}

func (d *DFSServerInstance) HeartbeatDisconnectExists(args *shared.Args, reply *shared.Reply) (err error) {
	_, exists := d.HeartbeatDisconnected[args.LocalPath]
	if !exists {
		reply.Exists = false
	} else {
		reply.Exists = true
	}
	return nil
}

// Returns true if client is connected
func (d *DFSServerInstance) IsConnected(args *shared.Args, reply *shared.Reply) (err error) {
	client := args.LocalPath
	if d.ConnectedClients[client] == "Connected" {
		reply.Connected = true
	} else {
		reply.Connected = false
	}
	return nil
}

// Adds a new file to server metadata at all the appropriate places
func (d *DFSServerInstance) UpdateServer(args *shared.Args, reply *shared.Reply) (err error) {
	// Add file to client's metadata
	clientMetadata := d.ClientInfo[args.LocalPath]

	// Create file and add to file list
	file := &shared.FileMetadata{
		Name:     args.Filename,
		Versions: args.Versions,
	}
	newFileList := append(clientMetadata.Files, file)

	// Reassign mapping
	updatedClientMetadata := &shared.ClientMetadata{
		ID:        clientMetadata.ID,
		Addr:      clientMetadata.Addr,
		LocalPath: clientMetadata.LocalPath,
		Files:     newFileList,
	}
	d.ClientInfo[args.LocalPath] = updatedClientMetadata

	// Update other Server metadata
	d.ClientFiles[args.LocalPath] = newFileList
	if len(d.FileVersions[args.Filename]) == 0 {
		d.FileVersions[args.Filename] = file.Versions
	}
	if len(d.Files[args.Filename]) > 0 {
		newList := append(d.Files[args.Filename], args.LocalPath)
		d.Files[args.Filename] = newList
	} else {
		d.Files[args.Filename] = []string{args.LocalPath}
	}

	return nil
}

// Returns chunk of file read or error
func (d *DFSServerInstance) Read(args *shared.Args, reply *shared.Reply) (err error) {
	//Get latest version of chunk
	versions := d.FileVersions[args.Filename]
	chunkVersion := versions[int(args.Chunknum)]

	// Find client with chunk
	var winner string
	clients := d.Files[args.Filename]
	var vers int
	if args.Mode == int(READ) && !args.Open {
		// Always has to return the latest version
		for _, client := range clients {
			if d.ConnectedClients[client] == "Connected" && client != args.LocalPath {
				files := d.ClientFiles[client]
				for _, file := range files {
					if file.Name == args.Filename {
						if file.Versions[int(args.Chunknum)] == chunkVersion {
							winner = client
							break
						}
					}
				}
			}
		}
	} else {
		// Get available latest version
		for vers = chunkVersion; vers >= 0; vers-- {
			for _, client := range clients {
				if d.ConnectedClients[client] == "Connected" {
					files := d.ClientFiles[client]
					for _, file := range files {
						if file.Name == args.Filename {
							if file.Versions[int(args.Chunknum)] == vers {
								winner = client
								break
							}
						}
					}
				}
			}
		}
	}

	if vers == 0 {
		return errors.New("Error because no trivial chunks allowed.")
	}

	if winner == "" {
		return errors.New("Error because no clients found.")
	}

	// Get chunk from client
	host := d.ClientInfo[winner]

	var clientReply shared.Reply
	clientArgs := &shared.Args{
		Filename:    args.Filename,
		LocalPath:   host.LocalPath,
		BytesToRead: args.BytesToRead,
		Offset:      args.Offset,
	}

	rpcConnection, _ := d.Clients[args.LocalPath]
	err = rpcConnection.Call("DFSServerInstance.GetChunk", clientArgs, &clientReply)
	if err != nil {
		return errors.New("Could not retrieve chunk from client")
	}

	//Update server about file version client has
	client := d.ClientInfo[args.LocalPath]
	files := client.Files
	var f *shared.FileMetadata
	var index int
	for in, file := range files {
		if file.Name == args.Filename {
			f = file
			index = in
			break
		}
	}
	versions = f.Versions
	versions[int(args.Chunknum)] = vers
	file := &shared.FileMetadata{
		Name:     f.Name,
		Versions: versions,
	}
	copyFiles := files
	copyFiles[index] = file
	newClient := &shared.ClientMetadata{
		ID:        client.ID,
		Addr:      client.Addr,
		LocalPath: client.LocalPath,
		Files:     copyFiles,
	}
	d.ClientInfo[args.LocalPath] = newClient
	d.ClientFiles[args.LocalPath] = copyFiles

	// Return version read, chunk copied
	reply.Chunk = clientReply.Chunk
	reply.Version = vers
	return nil
}

// Return version of chunks that client has
// If client is in write mode, nobody else can open the file
func (d *DFSServerInstance) Open(args *shared.Args, reply *shared.Reply) (err error) {
	filesList := d.ClientFiles[args.LocalPath]
	for _, file := range filesList {
		if file.Name == args.Filename {
			reply.Versions = file.Versions
		}

	}
	return nil
}

// Check latest version of file chunk
func (d *DFSServerInstance) LatestVersion(args *shared.Args, reply *shared.Reply) (err error) {
	versions := d.FileVersions[args.Filename]
	chunkVersion := versions[int(args.Chunknum)]
	reply.Version = chunkVersion
	return nil

}

// Return true if file exists in server
func (d *DFSServerInstance) GlobalFileExists(args *shared.Args, reply *shared.Reply) (err error) {
	filename := args.Filename
	_, exists := d.Files[filename]
	reply.Exists = exists
	return nil
}

// Return true if client with file exists otherwise false
func (d *DFSServerInstance) ClientFilesOnline(args *shared.Args, reply *shared.Reply) (err error) {
	clients := d.Files[args.Filename]
	exists := false
	if len(clients) > 0 {
		for _, client := range clients {
			if d.ConnectedClients[client] == "Connected" {
				exists = true
				break
			}
		}
	}
	reply.Exists = exists
	return nil
}

func (d *DFSServerInstance) UMountDFS(args *shared.Args, reply *shared.Reply) (err error) {
	d.ConnectedClients[args.LocalPath] = "Disconnected"
	if d.ConnectedClients[args.LocalPath] == "Connected" {
		d.HeartbeatServer.Close()
		client := d.Clients[args.LocalPath]
		client.Close()
		//d.Client.Close()
	}
	return nil
}
