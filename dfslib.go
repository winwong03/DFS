/*

This package specifies the application's interface to the distributed
file system (DFS) system to be used in assignment 2 of UBC CS 416
2017W2.

*/

package dfslib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"time"
	"unicode"
	//    "strings"
	"path/filepath"

	"../shared"
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

////////////////////////////////////////////////////////////////////////////////////////////
// <ERROR DEFINITIONS>

// These type definitions allow the application to explicitly check
// for the kind of error that occurred. Each API call below lists the
// errors that it is allowed to raise.
//
// Also see:
// https://blog.golang.org/error-handling-and-go
// https://blog.golang.org/errors-are-values

// Contains serverAddr
type DisconnectedError string

func (e DisconnectedError) Error() string {
	return fmt.Sprintf("DFS: Not connnected to server [%s]", string(e))
}

// Contains chunkNum that is unavailable
type ChunkUnavailableError uint8

func (e ChunkUnavailableError) Error() string {
	return fmt.Sprintf("DFS: Latest verson of chunk [%s] unavailable", string(e))
}

// Contains filename
type OpenWriteConflictError string

func (e OpenWriteConflictError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] is opened for writing by another client", string(e))
}

// Contains file mode that is bad.
type BadFileModeError FileMode

func (e BadFileModeError) Error() string {
	return fmt.Sprintf("DFS: Cannot perform this operation in current file mode [%s]", string(e))
}

// Contains filename
type BadFilenameError string

func (e BadFilenameError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] includes illegal characters or has the wrong length", string(e))
}

// Contains filename.
type WriteModeTimeoutError string

func (e WriteModeTimeoutError) Error() string {
	return fmt.Sprintf("DFS: Write access to filename [%s] has timed out; reopen the file", string(e))
}

// Contains filename
type FileUnavailableError string

func (e FileUnavailableError) Error() string {
	return fmt.Sprintf("DFS: Filename [%s] is unavailable", string(e))
}

// Contains local path
type LocalPathError string

func (e LocalPathError) Error() string {
	return fmt.Sprintf("DFS: Cannot access local path [%s]", string(e))
}

// Contains filename
type FileDoesNotExistError string

func (e FileDoesNotExistError) Error() string {
	return fmt.Sprintf("DFS: Cannot open file [%s] in D mode as it does not exist locally", string(e))
}

// </ERROR DEFINITIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Represents a file in the DFS system.
type DFSFile interface {
	// Reads chunk number chunkNum into storage pointed to by
	// chunk. Returns a non-nil error if the read was unsuccessful.
	//
	// Can return the following errors:
	// - DisconnectedError (in READ,WRITE modes)
	// - ChunkUnavailableError (in READ,WRITE modes)
	Read(chunkNum uint8, chunk *Chunk) (err error)

	// Writes chunk number chunkNum from storage pointed to by
	// chunk. Returns a non-nil error if the write was unsuccessful.
	//
	// Can return the following errors:
	// - BadFileModeError (in READ,DREAD modes)
	// - DisconnectedError (in WRITE mode)
	Write(chunkNum uint8, chunk *Chunk) (err error)

	// Closes the file/cleans up. Can return the following errors:
	// - DisconnectedError
	Close() (err error)
}

// Represents a connection to the DFS system.
type DFS interface {
	// Check if a file with filename fname exists locally (i.e.,
	// available for DREAD reads).
	//
	// Can return the following errors:
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	LocalFileExists(fname string) (exists bool, err error)

	// Check if a file with filename fname exists globally.
	//
	// Can return the following errors:
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	// - DisconnectedError
	GlobalFileExists(fname string) (exists bool, err error)

	// Opens a filename with name fname using mode. Creates the file
	// in READ/WRITE modes if it does not exist. Returns a handle to
	// the file through which other operations on this file can be
	// made.
	//
	// Can return the following errors:
	// - OpenWriteConflictError (in WRITE mode)
	// - DisconnectedError (in READ,WRITE modes)
	// - FileUnavailableError (in READ,WRITE modes)
	// - FileDoesNotExistError (in DREAD mode)
	// - BadFilenameError (if filename contains non alpha-numeric chars or is not 1-16 chars long)
	Open(fname string, mode FileMode) (f DFSFile, err error)

	// Disconnects from the server. Can return the following errors:
	// - DisconnectedError
	UMountDFS() (err error)
}

// </HELPER FUNCTIONS>
////////////////////////////////////////////////////////////////////////////////////////////

// Returns true if input is lowercase alphanumeric
func isAlphaNumeric(fname string) bool {
	for _, letter := range fname {
		if !unicode.IsLetter(letter) && !unicode.IsNumber(letter) {
			return false
		}
	}
	return true
}

// </CONCRETE TYPE>
////////////////////////////////////////////////////////////////////////////////////////////

// DFSInstance will do the RPC call using client.call("DFSServer.Method", args, reply)
type DFSInstance struct {
	Client      *rpc.Client // To call server
	Server      *rpc.Server // To receive msgs from server
	LocalPath   string
	IPAddr      string
	ServerAddr  string
	Connected   bool
	Heartbeat   *net.UDPConn
	FilesOpened []*OpenFile
}

func (dfs *DFSInstance) SendUDPHeartbeat(fname string) (exists bool, err error) {
	addr, _ := net.ResolveUDPAddr("udp", dfs.ServerAddr)
	myaddr, _ := net.ResolveUDPAddr("udp", dfs.IPAddr)

	conn, err := net.DialUDP("udp", myaddr, addr)
	dfs.Heartbeat = conn

	for {
		currentTime := time.Now()
		beat := &shared.Heartbeat{
			LocalPath: dfs.LocalPath,
			TimeSent:  currentTime,
		}
		msg, err := json.Marshal(beat)
		if err != nil {
			//fmt.Println("Error marshaling data")
		}

		//	msg := []byte("Hello there")

		_, err = conn.Write(msg)
		if err != nil {
			//fmt.Println("Error writing to UDP Conn")
		}

		time.Sleep((2*time.Second - time.Since(currentTime)) * time.Second)
	}
}

// Return true if file is on disk, else return false
func (dfs *DFSInstance) LocalFileExists(fname string) (exists bool, err error) {
	// Check arguments
	nameLength := len(fname)
	if nameLength < 1 || nameLength > 16 {
		return false, BadFilenameError(fname)
	}
	if !isAlphaNumeric(fname) {
		return false, BadFilenameError(fname)
	}

	// Check if file exists, needs .dfs ext
	ext := fmt.Sprintf("%s.dfs", fname)
	file := filepath.Join(dfs.LocalPath, ext)

	_, err = os.Stat(file)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// Return true if file exists, else return false
func (dfs *DFSInstance) GlobalFileExists(fname string) (exists bool, err error) {
	// Check arguments
	nameLength := len(fname)
	if nameLength < 1 || nameLength > 16 {
		return false, BadFilenameError(fname)
	}
	if !isAlphaNumeric(fname) {
		return false, BadFilenameError(fname)
	}

	var reply shared.Reply
	args := &shared.Args{
		Filename: fname,
	}

	err = dfs.Client.Call("DFSServerInstance.GlobalFileExists", args, &reply)
	if err != nil {
		//fmt.Println("this is the error from global file exists: ", err)
	}
	return reply.Exists, nil
	//return false, nil
}

// Client call to open a file for reading and writing
// Opens the file if already there otherwise creates one
func (dfs *DFSInstance) Open(fname string, mode FileMode) (f DFSFile, err error) {

	// Check validity of fname
	nameLength := len(fname)
	if nameLength < 1 || nameLength > 16 {
		return nil, BadFilenameError(fname)
	}
	if !isAlphaNumeric(fname) {
		return nil, BadFilenameError(fname)
	}

	var file *os.File
	var fileToChange *os.File
	var versions [256]int

	// Get full path of file
	ext := fmt.Sprintf("%s.dfs", fname)
	fullFilename := filepath.Join(dfs.LocalPath, ext)

	// Check if file exists locally
	exists, _ := dfs.LocalFileExists(fullFilename)

	// If DREAD, return FileDoesNotExistError if file doesnt exist
	if mode == DREAD {
		if !exists {
			return nil, FileDoesNotExistError(fname)
		}
	} else if mode == READ || mode == WRITE {
		//If disconnected return DisconnectedError
		if !dfs.Connected {
			return nil, DisconnectedError(dfs.ServerAddr)
		}

		// If mode == WRITE and somebody is already writing to file, return OpenWriteConflict
		// Otherwise tell server we're writing
		if mode == WRITE {
			var reply shared.Reply
			args := &shared.Args{
				LocalPath: dfs.LocalPath,
				Filename:  fname,
				Mode:      int(mode),
			}
			_ = dfs.Client.Call("DFSServerInstance.Writeable", args, &reply)
			if !reply.Writeable {
				return nil, OpenWriteConflictError(fname)
			}
		}

		// Check if log file exists. If not, create and tell server
		logFilePath := filepath.Join(dfs.LocalPath, "log.dfs")
		logFileExists, _ := dfs.LocalFileExists(logFilePath)

		if !logFileExists {
			// Create the file
			logFileName := filepath.Join(dfs.LocalPath, "log.dfs")
			log, err := os.Create(logFileName)
			log.Sync()

			var reply shared.Reply
			args := &shared.Args{
				Filename:  "log",
				LocalPath: dfs.LocalPath,
				Versions:  versions,
			}
			// Update server that file exists
			err = dfs.Client.Call("DFSServerInstance.UpdateServer", args, &reply)
			if err != nil {
				//fmt.Println("Could not update server") //delete
			}
		}

		// Check if global file exists and if client with file connected.
		//If not, return FileUnavailableError
		var reply shared.Reply
		args := &shared.Args{
			Filename:  fname,
			LocalPath: dfs.LocalPath,
		}
		err = dfs.Client.Call("DFSServerInstance.GlobalFileExists", args, &reply)
		globalExists := reply.Exists

		// If local file doesn't exist, create it
		if !exists {
			// Create the file
			fileToChange, err = os.Create(fullFilename)
			//fileToChange.Truncate(int64(256 * 32))
			fileToChange.Sync()
			fileToChange.Close()

			var reply shared.Reply
			args := &shared.Args{
				Filename:  fname,
				LocalPath: dfs.LocalPath,
				Versions:  versions,
			}
			// Update server that file exists
			err = dfs.Client.Call("DFSServerInstance.UpdateServer", args, &reply)
			if err != nil {
				//fmt.Println("Could not update server") //delete
			}
		}

		// Update local version if global and client files exists
		if globalExists {
			var reply shared.Reply
			err = dfs.Client.Call("DFSServerInstance.ClientFilesOnline", args, &reply)
			filesOnline := reply.Exists
			if !filesOnline {
				// Remove write access
				var reply shared.Reply
				args := &shared.Args{
					Filename:  fname,
					LocalPath: dfs.LocalPath,
				}
				err = dfs.Client.Call("DFSServerInstance.RemoveAccess", args, &reply)
				return nil, FileUnavailableError(fname)
			}

			// Since there are clients online, open file to get latest versions
			fileToChange, err = os.OpenFile(fullFilename, os.O_APPEND|os.O_RDWR, 0644)

			// Get our version of file to compare with latest
			reply = shared.Reply{}
			args = &shared.Args{
				Filename:  fname,
				LocalPath: dfs.LocalPath,
			}
			_ = dfs.Client.Call("DFSServerInstance.Open", args, &reply)
			versions = reply.Versions

			// Get the latest file from server
			// Update server that we have latest file
			reply = shared.Reply{}
			for index := 0; index < 256; index++ {
				// If not latest version, get chunk from server
				if reply.Version > versions[index] {
					var reply shared.Reply
					args := &shared.Args{
						Filename:    fname,
						LocalPath:   dfs.LocalPath,
						BytesToRead: 32,
						Offset:      index * 32,
						Chunknum:    uint8(index),
						Mode:        int(mode),
						Open:        true,
					}
					err = dfs.Client.Call("DFSServerInstance.Read", args, &reply)
					if err != nil {
						// Remove write access
						var reply shared.Reply
						args := &shared.Args{
							Filename: fname,
						}
						err = dfs.Client.Call("DFSServerInstance.RemoveAccess", args, &reply)
						return nil, ChunkUnavailableError(index)
					}

					// Save chunk to file
					_, err = fileToChange.WriteAt(reply.Chunk[:], int64(args.Offset))
					if err != nil {
					}
					// Save read part to disk
					fileToChange.Sync()
				}
			}

			// update versioning of local file
			reply = shared.Reply{}
			args = &shared.Args{
				Filename:  fname,
				LocalPath: dfs.LocalPath,
			}
			_ = dfs.Client.Call("DFSServerInstance.Open", args, &reply)
			versions = reply.Versions

			// Close the file to prevent opening twice
			fileToChange.Close()
		}
	}

	// Return instance of file
	file, err = os.OpenFile(fullFilename, os.O_APPEND|os.O_RDWR, 0644)
	if err != nil {
		// Remove write access
		var reply shared.Reply
		args := &shared.Args{
			Filename: fname,
		}
		err = dfs.Client.Call("DFSServerInstance.RemoveAccess", args, &reply)
		return nil, FileDoesNotExistError(fname)
	}

	openFile := &OpenFile{
		Name:      fname,
		File:      file,
		Mode:      mode,
		Connected: dfs.Connected,
		Server:    dfs.ServerAddr,
		Versions:  versions,
		Client:    dfs.Client,
		LocalPath: dfs.LocalPath,
	}

	dfs.FilesOpened = append(dfs.FilesOpened, openFile)

	return openFile, nil
}

func (dfs *DFSInstance) UMountDFS() (err error) {
	// Return DisconnectedError if not connected
	if !dfs.Connected {
		return DisconnectedError(dfs.ServerAddr)
	}

	// Close open files
	if dfs.FilesOpened != nil {
		for _, f := range dfs.FilesOpened {
			f.File.Close()
		}
	}
	dfs.FilesOpened = nil

	if dfs.Connected {
		var reply shared.Reply
		args := &shared.Args{
			LocalPath: dfs.LocalPath,
		}
		// Remove all writing access
		err = dfs.Client.Call("DFSServerInstance.RemoveAccess", args, &reply)

		// Change clients to disconnected and break RPC connections
		err = dfs.Client.Call("DFSServerInstance.UMountDFS", args, &reply)

		// Close connections
		dfs.Client.Close()
		dfs.Heartbeat.Close()
	}

	return nil
}

// File implements DFSFile interface
type OpenFile struct {
	Name      string
	File      *os.File //Actual file
	Mode      FileMode //Mode that file was opened with
	Connected bool     //if client gets disconnected while READ/WRITE, no future ops allowed
	Server    string
	Versions  [256]int //Version of each chunk
	Writes    []int    //Chunk nums written to

	Client    *rpc.Client // To call server, need to be able to update server at file close
	LocalPath string
}

//If disconnected, return DisconnectedError
// Return ChunkUnavailableError(chunk num)
func (f *OpenFile) Read(chunkNum uint8, chunk *Chunk) (err error) {
	// If disconnected, return DisconnectedError
	if !f.Connected {
		return DisconnectedError(f.Server)
	}

	// File can only have 256 chunks
	if int(chunkNum) < 0 || int(chunkNum) > 255 {
		return ChunkUnavailableError(chunkNum)
	}

	// Get offset into file
	seek := int64(chunkNum * 32)

	// if dread mode read from file
	if f.Mode == DREAD {
		_, err = f.File.ReadAt(chunk[:], seek)
		if err != nil {
			//fmt.Printf("error:%s\n", err) //delete
			return ChunkUnavailableError(chunkNum)
		}
	} else {
		// See if chunk version needs to be updated
		var reply shared.Reply
		args := &shared.Args{
			Filename: f.Name,
			Chunknum: chunkNum,
		}
		err = f.Client.Call("DFSServerInstance.LatestVersion", args, &reply)
		//fmt.Printf("LATEST VERSION ON SERVER:%d, OUR VERSION:%d\n", reply.Version, f.Versions[int(chunkNum)]) //delete

		// If it does, get chunk from server
		if reply.Version > f.Versions[int(chunkNum)] {
			var reply shared.Reply
			args := &shared.Args{
				Filename:    f.Name,
				LocalPath:   f.LocalPath,
				BytesToRead: 32,
				Offset:      int(seek),
				Chunknum:    chunkNum,
				Mode:        int(f.Mode),
				Open:        false,
			}
			// Get Chunk
			err = f.Client.Call("DFSServerInstance.Read", args, &reply)
			if err != nil {
				//fmt.Printf("1. chunk unavailable error:%s\n", err) //delete
				return ChunkUnavailableError(chunkNum)
			}

			// Update file and version read in file metadata
			f.Versions[int(chunkNum)] = reply.Version
			_, err = f.File.WriteAt(reply.Chunk[:], seek)
			if err != nil {
				//fmt.Println("Error occured writing chunk to file")
			}
			// Save read part to disk
			f.File.Truncate(int64(256 * 32))
			f.File.Sync()
		}
	}

	n, err := f.File.ReadAt(chunk[:], seek)
	count := 0
	if err != nil {
		for _, byte := range chunk[:] {
			if byte != 0 {
				count += 1
			}
		}
		// Chunk is empty
		if n == 0 && count == 0 {
			return nil
		}

		// Chunk is not empty but we reached EOF
		if count > 0 {
			_, err := f.File.ReadAt(chunk[:count], seek)
			if err != nil {
				return nil
			}

		}
		return ChunkUnavailableError(chunkNum)
	}
	return nil
}

func (f *OpenFile) Write(chunkNum uint8, chunk *Chunk) (err error) {
	// If disconnected, return DisconnectedError
	if !f.Connected {
		return DisconnectedError(f.Server)
	}

	//if mode is READ or DREAD, return BadFileMode
	if f.Mode == READ || f.Mode == DREAD {
		return BadFileModeError(f.Mode)
	}

	var reply shared.Reply
	args := &shared.Args{
		LocalPath: f.LocalPath,
	}
	err = f.Client.Call("DFSServerInstance.HeartbeatDisconnectExist", args, &reply)
	if reply.Exists {
		return WriteModeTimeoutError(f.Name)
	}

	// Update log that it needs to write
	ext := fmt.Sprintf("%s.dfs", "log")
	log := filepath.Join(f.LocalPath, ext)
	logFile, err := os.OpenFile(log, os.O_APPEND|os.O_RDWR, 0644)
	defer logFile.Close()
	if err != nil {
		//fmt.Print("Error opening log file to record write.") //delete
	}

	// Actual number of things to write
	count := 0
	for _, byte := range chunk[:] {
		if byte != 0 {
			count += 1
		}
	}

	msg := fmt.Sprintf("WRITING: %d, %s\n", int(chunkNum), (chunk[:count]))
	_, err = logFile.WriteString(msg)
	logFile.Sync()

	// Write
	offset := int64(chunkNum * 32)
	f.File.WriteAt(chunk[:count], offset)
	f.File.Sync()

	//Update log that write complete, but needs to update server
	msg = fmt.Sprintf("SERVER FILES UPDATING: %d, %s\n", int(chunkNum), chunk[:count])
	_, err = logFile.WriteString(msg)
	logFile.Sync()

	// Update Server
	reply = shared.Reply{}
	args = &shared.Args{
		Filename:  f.Name,
		LocalPath: f.LocalPath,
		Chunknum:  chunkNum,
	}
	err = f.Client.Call("DFSServerInstance.UpdateChunkVersion", args, &reply)

	// Update self
	version := reply.Version
	newVersions := f.Versions
	newVersions[int(chunkNum)] = version
	f.Versions = newVersions

	// Update log that write complete
	msg = fmt.Sprintf("WRITE COMPLETE\n")
	_, err = logFile.WriteString(msg)
	logFile.Sync()

	return nil

}

func (f *OpenFile) Close() (err error) {
	// If Mode = READ/WRITE and disconnected, return DisconnectedError
	if !f.Connected && f.Mode != DREAD {
		return DisconnectedError(f.Server)
	}

	if f.Mode == WRITE && f.Connected {
		var reply shared.Reply
		args := &shared.Args{
			Filename:  f.Name,
			LocalPath: f.LocalPath,
		}
		// If WRITE, remove writing access block
		err = f.Client.Call("DFSServerInstance.RemoveAccess", args, &reply)
	}

	f.File.Close()
	return nil
}

// The constructor for a new DFS object instance. Takes the server's
// IP:port address string as parameter, the localIP to use to
// establish the connection to the server, and a localPath path on the
// local filesystem where the client has allocated storage (and
// possibly existing state) for this DFS.
//
// The returned dfs instance is singleton: an application is expected
// to interact with just one dfs at a time.
//
// This call should succeed regardless of whether the server is
// reachable. Otherwise, applications cannot access (local) files
// while disconnected.
//
// Can return the following errors:
// - LocalPathError
// - Networking errors related to localIP or serverAddr

func MountDFS(serverAddr string, localIP string, localPath string) (dfs DFS, err error) {
	//TODO if client reconnects, need to update server - look through for disconnected servers with same local path and create a file with unique id and during mount, check that localPath for that file

	// Same client has unique local paths, different clients can have same local patho
	// Get disconnected clients
	// Compare name
	// Compare files in local path

	connected := true

	// If path does not exist, return LocalPathError
	_, err = os.Stat(localPath)
	if err != nil {
		return nil, LocalPathError(localPath)
	}

	// Get addresses
	server, _ := net.ResolveTCPAddr("tcp", serverAddr)
	formatted := fmt.Sprintf("%s:0", localIP)
	local, err := net.ResolveTCPAddr("tcp", formatted)

	// Connect to the server, ignore connection errors
	//fmt.Printf("These are the things: local:%s, server:%s\n", local, server)
	conn, err := net.DialTCP("tcp", local, server)

	var dfsClient *DFSInstance
	if err != nil {
		// If it can't connect to server, then it's disconnected
		//fmt.Println("Couldn't connect to tcp: ", err)
		connected = false
	} else {
		//Otherwise
		// Get client address of connection
		addr := conn.LocalAddr()

		// Listen for rpc messages from RPC server
		dfsInstance := new(ClientInstance)
		rpcServer := rpc.NewServer()
		rpcServer.Register(dfsInstance)

		//str := fmt.Sprintf("%s:0", localIP)
		//hhp, err := net.ResolveTCPAddr("tcp", str)
		listener, err := net.ListenTCP("tcp", local)
		if err != nil {
			//fmt.Println("Error occurred listening to rpc client: ", err) //delete connected = false
			return nil, err

		}

		//go rpcServer.Accept(listener)
		// Serve in another goroutine
		go func() {
			for {
				// Accept connections and block until listener receives non-nil error
				accept, err := listener.Accept()
				if err != nil {
					continue
				}
				go rpcServer.ServeConn(accept)
			}
		}()

		// Register self as a DFS Client Instance
		dfsClient = &DFSInstance{
			Client:     rpc.NewClient(conn),
			LocalPath:  localPath,
			IPAddr:     addr.String(),
			ServerAddr: serverAddr,
			Server:     rpcServer,
			Connected:  connected,
		}

		// Check if it has mounted before

		// Register client to server
		var reply shared.Reply
		args := &shared.Args{
			LocalPath:  localPath,
			Addr:       addr.String(),
			ServerAddr: serverAddr,
		}
		err = dfsClient.Client.Call("DFSServerInstance.Mount", args, &reply)
		if err != nil {
			//fmt.Println("error occurred during rpc mounting call") //delete
			dfsClient.Connected = false
			//return nil, err
		}

		/* ignore this
		//fmt.Printf("Client %d is Connected\n", reply.ID) //delete
		// Get new assigned LocalPath
		dfsClient.LocalPath = reply.LocalPath

		// Create a local path for the client
		//os.Mkdir(dfsClient.LocalPath, 0700)
		*/

		// Start sending UDP Heartbeat
		go dfsClient.SendUDPHeartbeat(localPath)

		return dfsClient, nil
	}

	// When not connected
	// Register self as a DFS Client Instance
	dfsClient = &DFSInstance{
		LocalPath:  localPath,
		ServerAddr: serverAddr,
		Connected:  false,
	}
	return dfsClient, nil
}

type ClientInstance int

// Returns a chunk of file from client
func (d *ClientInstance) GetChunk(args *shared.Args, reply *shared.Reply) (err error) {
	// Open file
	ext := fmt.Sprintf("%s.dfs", args.Filename)
	fullPath := filepath.Join(args.LocalPath, ext)
	file, err := os.Open(fullPath)
	if err != nil {
		//fmt.Println("Error opening file")
		return errors.New("Error opening file")

	}
	chunk := make([]byte, 32)
	_, err = file.ReadAt(chunk, int64(args.Offset))
	if err != nil {
		//fmt.Println("There was a problem reading the chunk:", err) //delete
		return errors.New("Error reading chunk")
	}
	// Return data read
	copy(reply.Chunk[:], chunk)
	return nil
}
