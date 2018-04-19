/*
 * This program tests the following scenerio:
 * Client A, B, C are connected to server
 * Client A opens file foo for WRITING
 * Client A writes to chunk 0 of foo
 * Client A closes file
 * Client B opens file foo for WRITING
 * Client B writes to chunk 1 of foo
 * Client B closes file
 * Client C open file foo for READING and observes both changes of A and B
 * Client A opens file foo for READING and observes both changes of A and B
 */

package main

import(
	"./dfslib"
	"io/ioutil"
	"os"
	"fmt"
)


func main(){
	if len(os.Args) != 2{
		fmt.Println("Usage: go run multiple_write_app.go [server host:ip]")
		return
	}
	serverAddr := os.Args[1]
	localIP := "127.0.0.1" // you may want to change this when testing

	clientALocalPath, errA := ioutil.TempDir(".", "clientA")
	clientBLocalPath, errB := ioutil.TempDir(".", "clientB")
	clientCLocalPath, errC := ioutil.TempDir(".", "clientC")
	if errA != nil || errB != nil ||errC != nil {
		panic("Could not create temporary directory")
	}

	clientA, errA := dfslib.MountDFS(serverAddr, localIP, clientALocalPath)
	clientB, errB := dfslib.MountDFS(serverAddr, localIP, clientBLocalPath)
	clientC, errC := dfslib.MountDFS(serverAddr, localIP, clientCLocalPath)
	if errA != nil|| errB != nil || errC != nil {
		fmt.Println("Error: Could not mount clients")
	}

	f_a, err := clientA.Open("foo", dfslib.WRITE)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Create a chunk with a string message.
	var chunk dfslib.Chunk
	const str = "Hello from client A!"
	copy(chunk[:], str)


	f_a.Write(0, &chunk)

	f_a.Close()

	f_a , err = clientB.Open("foo", dfslib.WRITE)
	if err != nil {
		fmt.Println(err)
	}

	copy(chunk[:], "Hello from client B")

	err = f_a.Write(1, &chunk)
	if err != nil {
		fmt.Println("Error: Client B could not write to file a.", err)
		return
	}

	f_a.Close()

	f_a2, err := clientC.Open("foo", dfslib.READ)
	if err != nil {
		fmt.Println(err)
		return
	}

	err = f_a2.Read(0, &chunk)
	if err != nil {
		fmt.Println("Client C could not read CHUNK 0 from foo. ", err)
		return
	}

	fmt.Println( "Client C read CHUNK 0 from foo. ", string(chunk[:]))

	err = f_a2.Read(1, &chunk)
	if err != nil {
		fmt.Println("Client C could not read CHUNK 1 from foo. ", err)
		return
	}

	fmt.Println( "Client C read CHUNK 1 from foo. ", string(chunk[:]))

	f_a3, err := clientA.Open("foo", dfslib.READ)
	if err != nil {
		fmt.Println("Error: Client A could not open foo." , err)
	}

	err = f_a3.Read(0, &chunk)
	if err != nil {
		fmt.Println("Client A could not read CHUNK 0 from foo. ", err)
		return
	}

	fmt.Println( "Client A read CHUNK 0 from foo. ", string(chunk[:]))

	err = f_a3.Read(1, &chunk)
	if err != nil {
		fmt.Println("Client A could not read CHUNK 1 from foo. ", err)
		return
	}

	fmt.Println( "Client A read CHUNK 1 from foo. ", string(chunk[:]))


}