/*

A trivial application to illustrate how the dfslib library can be used
from an application in assignment 2 for UBC CS 416 2017W2.

Usage:
go run app.go
*/

package main

// Expects dfslib.go to be in the ./dfslib/ dir, relative to
// this app.go file
import (
	"fmt"
	"os"

	"./dfslib"
)

func main() {
	serverAddr := "127.0.0.1:8080"
	localIP := "127.0.0.1"
	localPath := "/tmp/dfs-dev/"

	// Connect to DFS.
	dfs, err := dfslib.MountDFS(serverAddr, localIP, localPath)
	if checkError(err) != nil {
		fmt.Println("Error with MountDFS")
		return
	}

	// Close the DFS on exit.
	// Defers are really cool, check out: https://blog.golang.org/defer-panic-and-recover
	defer dfs.UMountDFS()

	// Check if hello.txt file exists in local space
	/*
		lexists, err := dfs.LocalFileExists("helloworld")
		lexists, err := dfs.GlobalFileExists("helloworld")
		if checkError(err) != nil {
			return
		}

		if lexists {
			fmt.Println("File already exists, mission accomplished")
			return
		}
	*/

	// Check if hello.txt file exists in the global DFS.
	exists, err := dfs.GlobalFileExists("helloworld")
	if checkError(err) != nil {
		fmt.Println("Error with GlobalFileExists")
		return
	}

	if exists {
		fmt.Println("File already exists, mission accomplished")
		return
	}

	// Open the file (and create it if it does not exist) for writing.
	f, err := dfs.Open("helloworld", dfslib.WRITE)
	if checkError(err) != nil {
		fmt.Println("\nOpen error")
		return
	}

	/*
		// Check if hello.txt file exists in the global DFS.
		exists, err = dfs.GlobalFileExists("helloworld")
		if checkError(err) != nil {
			fmt.Println("Error with GlobalFileExists")
			return
		}
		if exists {
			fmt.Println("Global File already exists, mission accomplished")
		}

		// Check if hello.txt file exists in the global DFS.
		exists, err = dfs.LocalFileExists("helloworld")
		if checkError(err) != nil {
			fmt.Println("Error with LocalFileExists")
			return
		}
		if exists {
			fmt.Println("Local File already exists, mission accomplished")
		} else {
			fmt.Println("Local File does not exist")
		}
	*/

	// Close the file on exit.
	defer f.Close()

	// Create a chunk with a string message.
	var chunk dfslib.Chunk
	var chunk2 dfslib.Chunk
	const str = "Hello friends!"
	copy(chunk[:], str)

	// Write the 0th chunk of the file.
	err = f.Write(0, &chunk)
	if checkError(err) != nil {
		fmt.Println("Write error")
		return
	}

	// Read the 0th chunk of the file.
	fmt.Printf("f:%v\n", f)
	err = f.Read(0, &chunk2)
	if checkError(err) != nil {
		fmt.Println("Read error")
		return
	} else {
		fmt.Printf("This is the file contents:[%s]\n", chunk2)
		fmt.Println("ALL DONE")
	}

	for {
	}
}

// If error is non-nil, print it out and return it.
func checkError(err error) error {
	if err != nil {
		fmt.Fprint(os.Stderr, "Error ", err.Error())
		return err
	}
	return nil
}
