package main

import (
	"bufio"
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

// For now, we will not only hardcode and use our local machine as the worker
var workers = []string{
	"192.168.0.238:50051",
	"192.168.0.241:50051",
	"192.168.0.207:50051",
}

const chunkSize = 50 * 1024 * 1024

func main() {
	log.Println("[MASTER] Starting master server...")

	filename := flag.String("file", "", "Path to the log file")
	// maybe add workers flag to specify worker addresses
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

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	// 1. Split the file into chunks
	// For example, we might read a large log file in 1MB increments or parse line by line.
	// chunkData, err := splitFile("path/to/file.log")

	// 2. Distribute chunks to workers
	var allPartialResults []*pb.PartialResult

	var wg sync.WaitGroup
	chunkID := 0
	workerIndex := 0
	buffer := make([]byte, chunkSize)
	for {
		n, err := file.Read(buffer)
		if n > 0 {
			dataChunk := make([]byte, n)
			copy(dataChunk, buffer[:n])

			// Create and send chunk to worker
			go sendChunk(chunkID, dataChunk, workers[workerIndex%len(workers)], &wg)
			chunkID++
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Failed to read chunk: %v", err)
		}
	}
	log.Print("Finished reading the file")
/* 	if len(chunk) > 0 {
		wg.Add(1)
		currentChunk := make([]string, len(chunk))
		copy(currentChunk, chunk)
		currentWorker := workers[workerIndex%len(workers)]
		go sendChunk(chunkID, currentChunk, currentWorker, &wg)
		chunkID++
	} */

	if err := scanner.Err(); err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}


	wg.Wait()
	log.Printf("[MASTER] Received all partial results: %v", allPartialResults)
	// 3. Aggregate partial results (reduce) - either locally or by calling the ProcessReduce 
	// Lets do it locally for now and simplicity
	aggregate := localReduce(allPartialResults)
	log.Printf("[MASTER] Final Aggregated results: %v", aggregate)
	// ALTERNATIVELY, you could call ProcessReduce on a worker
	// reduceResp, err := client.ProcessReduce(context.Background(), &pb.ReduceRequest{
	// 	PartialResults: allPartialResults,
	// })

	// Save or output the final aggregated results
}

func trimSpace(s string) string {
	return string([]byte(s))
}

func sendChunk(id int, lines []byte, workerAddr string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.Dial(workerAddr, grpc.WithInsecure(), grpc.WithDefaultCallOptions(
		grpc.MaxCallRecvMsgSize(1024*1024*1024),
		grpc.MaxCallSendMsgSize(1024*1024*1024),
	))
	if err != nil {
		log.Fatalf("Failed to dial worker %s: %v", workerAddr, err)
	}
	defer conn.Close()

	client := pb.NewMapReduceServiceClient(conn)


	req := &pb.MapRequest{
		ChunkId: fmt.Sprintf("chunk-%d", id),
		LogData: lines,
	}

	resp, err := client.ProcessMap(context.Background(), req)
	if err != nil {
		log.Fatalf("Failed to process map: %v", err)
		return
	}
	log.Printf("Chunk %d processed by worker %s, with %d partial results", id, workerAddr, len(resp.PartialResults))
	log.Printf("Partial results: %v", localReduce(resp.PartialResults))
}


func localReduce(partials []*pb.PartialResult) map[string]int64 {
	counts := make(map[string]int64)
	for _, pr := range partials {
		counts[pr.Key] += pr.Count
	}
	return counts
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result
}