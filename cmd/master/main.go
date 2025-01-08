package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	pb "gitlab.betv3.xyz/BetV3/distributed-key-value-store/internal/grpc"
	"google.golang.org/grpc"
)

// For now, we will hardcode the worker addresses
var workers = []string{
	"127.0.0.1:50051",
	// add more workers here
}

const chunkSize = 50 * 1024 * 1024

func main() {
	log.Println("[MASTER] Starting master server...")
	// Parse the command line flags
	filename := flag.String("file", "", "Path to the log file")
	flag.Parse()

	//validate the filename
	if *filename == "" {
		log.Fatal("Please provide a log file using -file flag")
		flag.Usage()
		os.Exit(1)
	}

	// Process the log file
	fmt.Printf("Processing log file: %s\n", *filename)
	file, err := os.Open(*filename)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()
	// Create a buffered 10MB scanner
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	// Instantiate partialResults variable to store all partial results
	var (
		allPartialResults []*pb.PartialResult
		mu                sync.Mutex
	)
	// Create a WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup
	// instantiate chunkID and workerIndex variables
	chunkID := 0
	workerIndex := 0
	// create a buffer to store the chunk data
	buffer := make([]byte, chunkSize)
	leftover := make([]byte, 0)
	// Read the file in chunks and send each chunk to a worker
	for {
		n, err := file.Read(buffer)
		// Break if we didnt read anything
		if n == 0 {
			break
		}
		// add delta to the waitgroup counter
		wg.Add(1)
		// append the leftover data from the previous chunk
		data := append(leftover, buffer[:n]...)
		// find the last newline character in the chunk
		lastNewline := bytes.LastIndexByte(data, '\n')
		if lastNewline == -1 {
			lastNewline = len(data)
		}
		// split the chunk at the last newline character
		chunk := data[:lastNewline]
		// store the leftover data for the next chunk
		leftover = data[lastNewline+1:]
		// send the chunk to a worker
		go sendChunk(chunkID, chunk, workers[workerIndex%len(workers)], &wg, &allPartialResults, &mu)
		chunkID++
		workerIndex++
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to read chunk: %v", err)
		}
	}
	log.Print("Finished reading the file")

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Wait for all workers to finish
	wg.Wait()
	log.Printf("[MASTER] Received all partial results: %v", allPartialResults)
	aggregate := localReduce(allPartialResults)
	log.Printf("[MASTER] Final Aggregated results: %v", aggregate)
}


func sendChunk(id int, lines []byte, workerAddr string, wg *sync.WaitGroup, allPartialResults *[]*pb.PartialResult, mu *sync.Mutex) {
	defer wg.Done()
	// Connect to the worker
	conn, err := grpc.Dial(workerAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(1024*1024*1024),
		grpc.MaxCallSendMsgSize(1024*1024*1024),
	))
	if err != nil {
		log.Fatalf("Failed to dial worker %s: %v", workerAddr, err)
	}
	defer conn.Close()
	// Create a client
	client := pb.NewMapReduceServiceClient(conn)

	// Create the request for the worker
	req := &pb.MapRequest{
		ChunkId: fmt.Sprintf("chunk-%d", id),
		LogData: lines,
	}
	// Send the request to the worker
	resp, err := client.ProcessMap(context.Background(), req)
	if err != nil {
		log.Fatalf("Failed to process map: %v", err)
		return
	}
	// Append the partial results to the allPartialResults slice
	mu.Lock()
	*allPartialResults = append(*allPartialResults, resp.PartialResults...)
	mu.Unlock()
}


// localReduce aggregates partial results from multiple sources into a single
// map. It takes a slice of PartialResult pointers and returns a map where the
// keys are the same as the PartialResult keys and the values are the sum of
// the counts for each key.
//
// Parameters:
//   partials - a slice of pointers to PartialResult, each containing a key and
//              a count.
//
// Returns:
//   A map where each key is a string from the PartialResult and the value is
//   the total count for that key.
func localReduce(partials []*pb.PartialResult) map[string]int64 {
	counts := make(map[string]int64)
	for _, pr := range partials {
		counts[pr.Key] += pr.Count
	}
	return counts
}