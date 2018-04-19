package shared

import "time"

type Args struct {
	LocalPath   string
	Addr        string
	Filename    string
	Version     int
	BytesToRead int
	Offset      int
	Versions    [256]int
	Mode        int
	Chunknum    uint8
	ServerAddr  string
	Open        bool
}

// Reply struct
type Reply struct {
	ID        int
	LocalPath string
	Exists    bool
	Connected bool
	Filename  string
	Version   int
	Versions  [256]int
	Chunk     [32]byte
	Writeable bool
}

type Heartbeat struct {
	LocalPath string
	TimeSent  time.Time
}

// Information about Files
type FileMetadata struct {
	Name     string            //Name of file
	Versions [256]int          //Version of each chunk
	Access   map[string]string //Record client writing to file writing
}

//Information about Client
type ClientMetadata struct {
	ID        int             //ID of client
	Addr      string          //IP address of client
	LocalPath string          //LocalPath of client
	Files     []*FileMetadata //List of files that client has
}

/*
// Argument struct
type Args struct {
	LocalPath   string
	Addr        string
	Filename    string
	Version     int
	BytesToRead int
	Offset      int
	Versions    [256]int
	Mode        int
	Chunknum    uint8
}

// Reply struct
type Reply struct {
	ID        int
	LocalPath string
	Exists    bool
	Connected bool
	Filename  string
	Version   int
	Versions  [256]int
	Chunk     [32]byte
	Writeable bool
}

// DFSServer interface for rpc calls
type DFSServer interface {
	Mount(args *Args, reply *Reply) error
	LocalFileExists(args *Args, reply *bool) error
	GlobalFileExists(args *Args, reply *bool) error
	Open(args *Args, reply *Reply) error
	IsConnected(args *Args, reply *Reply) error
	GetFileVersion(args *Args, reply *Reply) error
	UpdateServer(args *Args, reply *Reply) error
	GetChunk(args *Args, reply *Reply) error
	UmountDFS(args *Args, reply *bool) error
}

// Test Data Structures
type Args1 struct {
	ID string
}

type Reply1 struct {
	Res bool
}
*/
