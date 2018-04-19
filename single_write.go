// CPSC 416 | 2017W2 | Assignment 2
//
// This tests a simple, single client DFS scenario. Two clients (A and B) connect
// to the same server in turns:
//
// - Client A mounts DFS at a certain local path (see main function)
// - Client B mounts DFS at a certain local path (see main function)
// - Client A opens a new file for writing 
// - Client B opens a new file for reading 
// - Client A writes some content on chunk 3
// - Client B reads that file on that chunk
//
// Usage:
//
// $ ./single_client [server-address]

package main

import (
	"./dfslib"

	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

const (
	CHUNKNUM          = 3                           // which chunk client A will try to read from and write to
	VALID_FILE_NAME   = "cpsc416"                   // a file name client A will create
	INVALID_FILE_NAME = "invalid file;"             // a file name that the dfslib rejects
	DEADLINE          = "2018-01-29T23:59:59-08:00" // project deadline :-)
)

//////////////////////////////////////////////////////////////////////
// helper functions -- no need to look at these
type testLogger struct {
	prefix string
}

func NewLogger(prefix string) testLogger {
	return testLogger{prefix: prefix}
}

func (l testLogger) log(message string) {
	fmt.Printf("[%s][%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), l.prefix, message)
}

func (l testLogger) TestResult(description string, success bool) {
	var label string
	if success {
		label = "OK"
	} else {
		label = "ERROR"
	}

	l.log(fmt.Sprintf("%-70s%-10s", description, label))
}

func usage() {
	fmt.Fprintf(os.Stderr, "%s [server-address]\n", os.Args[0])
	os.Exit(1)
}

func reportError(err error) {
	timeWarning := []string{}

	deadlineTime, _ := time.Parse(time.RFC3339, DEADLINE)
	timeLeft := deadlineTime.Sub(time.Now())
	totalHours := timeLeft.Hours()
	daysLeft := int(totalHours / 24)
	hoursLeft := int(totalHours) - 24*daysLeft

	if daysLeft > 0 {
		timeWarning = append(timeWarning, fmt.Sprintf("%d days", daysLeft))
	}

	if hoursLeft > 0 {
		timeWarning = append(timeWarning, fmt.Sprintf("%d hours", hoursLeft))
	}

	timeWarning = append(timeWarning, fmt.Sprintf("%d minutes", int(timeLeft.Minutes())-60*int(totalHours)))
	warning := strings.Join(timeWarning, ", ")

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	fmt.Fprintf(os.Stderr, "\nPlease fix the bug above and run this test again. Time remaining before deadline: %s\n", warning)
	os.Exit(1)
}

//////////////////////////////////////////////////////////////////////


func single_write(serverAddr, localIP, localPath1 string, localPath2 string) (err error) {
	var blob dfslib.Chunk
	var dfs1 dfslib.DFS
	var dfs2 dfslib.DFS

	logger1 := NewLogger("Client A")
	logger2 := NewLogger("Client B")

    /////////////////// Client A mounts

	testCase1 := fmt.Sprintf("Mounting DFS('%s', '%s', '%s')", serverAddr, localIP, localPath1)
	dfs1, err = dfslib.MountDFS(serverAddr, localIP, localPath1)
	if err != nil {
		logger1.TestResult(testCase1, false)
		return
	}
	logger1.TestResult(testCase1, true)
	defer func() {
		// if the client is ending with an error, do not make thing worse by issuing
		// extra calls to the server
		if err != nil {
			return
		}

		if err = dfs1.UMountDFS(); err != nil {
			logger1.TestResult("Unmounting DFS", false)
			return
		}

		logger1.TestResult("Unmounting DFS", true)
	}()

    ////////////////// Client 2 mounts

	testCase2 := fmt.Sprintf("Mounting DFS('%s', '%s', '%s')", serverAddr, localIP, localPath2)
	dfs2, err = dfslib.MountDFS(serverAddr, localIP, localPath2)
	if err != nil {
		logger2.TestResult(testCase2, false)
		return
	}
	logger2.TestResult(testCase2, true)
	defer func() {
		// if the client is ending with an error, do not make thing worse by issuing
		// extra calls to the server
		if err != nil {
			return
		}
		if err = dfs2.UMountDFS(); err != nil {
			logger2.TestResult("Unmounting DFS", false)
			return
		}
		logger2.TestResult("Unmounting DFS", true)
	}()

    ////////////// Client A opens for write

	testCase1 = fmt.Sprintf("Opening file '%s' for writing", VALID_FILE_NAME)
	file1, err := dfs1.Open(VALID_FILE_NAME, dfslib.WRITE)
	if err != nil {
		logger1.TestResult(testCase1, false)
		return
	}
	defer func() {
		if err != nil {
			return
		}
		testCase1 := fmt.Sprintf("Closing file '%s'", VALID_FILE_NAME)
		err = file1.Close()
		if err != nil {
			logger1.TestResult(testCase1, false)
			return
		}
		logger1.TestResult(testCase1, true)
	}()
	logger1.TestResult(testCase1, true)

    ////////////////// Client B opens file for read

	testCase2 = fmt.Sprintf("Opening file '%s' for reading succeeds", VALID_FILE_NAME)

	file2, err := dfs2.Open(VALID_FILE_NAME, dfslib.READ)
	if err != nil {
		logger2.TestResult(testCase2, false)
		err = fmt.Errorf("Expected opening file '%s' to succeed, but it failed", VALID_FILE_NAME)
		return
	}
	defer func() {
		if err != nil {
			return
		}
		testCase2 := fmt.Sprintf("Closing file '%s'", VALID_FILE_NAME)
		err = file2.Close()
		if err != nil {
			logger2.TestResult(testCase2, false)
			return
		}
		logger2.TestResult(testCase2, true)
	}()

	logger2.TestResult(testCase2, true)
	err = nil // so that the main function won't report the above (expected) error

    //////////////// Client A writes some content on Chunk3

	testCase1 = fmt.Sprintf("Writing chunk %d", 3)
    content1 := "Writing some content"

	copy(blob[:], content1)
	err = file1.Write(3, &blob)
	if err != nil {
		logger1.TestResult(testCase1, false)
		return
	}
	logger1.TestResult(testCase1, true)

    ////////////////// Client B reads the write

	testCase2 = fmt.Sprintf("Able to read '%s' back from chunk %d", content1, 3)

    blob = dfslib.Chunk{} 
	err = file2.Read(3, &blob)
	if err != nil {
		logger2.TestResult(testCase2, false)
		return
	}
	str := string(blob[:len(content1)])
	if str != content1 {
		logger2.TestResult(testCase2, false)
		return fmt.Errorf("Reading from chunk %d. Expected: '%s'; got: '%s'", 3, content1, str)
	}
	logger2.TestResult(testCase2, true)
    
    return
}

func main() {
	// usage: ./single_client [server-address]
	if len(os.Args) != 2 {
		usage()
	}

	serverAddr := os.Args[1]
	localIP := "198.162.33.23"

	// this creates a directory (to be used as localPath) for each client.
	// The directories will have the format "./client{A,B}NNNNNNNNN", where
	// N is an arbitrary number. Feel free to change these local paths
	// to best fit your environment
	clientALocalPath, errA := ioutil.TempDir(".", "clientA")
	clientBLocalPath, errB := ioutil.TempDir(".", "clientB")
	if errA != nil || errB != nil {
		panic("Could not create temporary directory")
	}

	if err := single_write(serverAddr, localIP, clientALocalPath, clientBLocalPath); err != nil {
		reportError(err)
	}

	fmt.Printf("\nCONGRATULATIONS! Your DFS implementation correctly handles the single write scenario.\n")
}
