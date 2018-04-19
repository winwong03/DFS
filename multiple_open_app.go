/*
 * This program tests the following scenerio:
 * Client A, B, C are connected to server
 * Client A opens file foo for WRITING
 * Client B and C call GlobalFileExists on foo SUCCEEDS
 * Client B attempt to open file foo for WRITING and fails
 * Client A closes file
 * Client B and C both open file foo for READING
 * Client B and C both successfully read file
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
		fmt.Println("Usage: go run multiple_open_app.go [server host:ip]")
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

	exists, err := clientB.GlobalFileExists("foo")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Does foo exit globally for client B?", exists)

	exists, err = clientC.GlobalFileExists("foo")

	fmt.Println("Does filec exit globally for client C?", exists)

	_, err = clientB.Open("foo", dfslib.WRITE)
	if err != nil {
		// ERROR IS EXPECTED BECAUSE CLIENT A IS CURRENTLY WRITING
		fmt.Println(err)
	}

	err = f_a.Close()
	if err != nil {
		fmt.Println("Error closing file.", err)
		return
	}

	err = f_a.Read(0, &chunk)
	if err != nil {
		fmt.Println(err)
	}

	f_a1 ,err := clientB.Open("foo", dfslib.READ)
	if err != nil {
		fmt.Println(err)
		return
	}

	f_a2, err := clientC.Open("foo", dfslib.READ)
	if err != nil {
		fmt.Println(err)
		return
	}

	//copy(chunk[:], "Hello from client B!")
	err = f_a1.Read(0, &chunk)
	if err != nil {
		fmt.Println("Client B could not read from foo. ", err)
		return
	}

	fmt.Println("Client B read from foo. ", string(chunk[:]))

	err = f_a2.Read(0, &chunk)
	if err != nil {
		fmt.Println("Client C could not read from foo. ", err)
		return
	}

	fmt.Println("Client C read from foo. ", string(chunk[:]))




}