package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"
	"bufio"

	pb "gitlab.betv3.xyz/BetV3/distributed-key-value-store/internal/grpc"
	"google.golang.org/grpc"
)

// For now, we will not only hardcode and use our local machine as the worker
var workers = []string{
	"localhost:50051",
}

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
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	// 1. Split the file into chunks
	// For example, we might read a large log file in 1MB increments or parse line by line.
	// chunkData, err := splitFile("path/to/file.log")

	// 2. Distribute chunks to workers
	var allPartialResults []*pb.PartialResult

	var wg sync.WaitGroup
	chunk := []string{}
	chunkID := 0
	workerIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		chunk = append(chunk, line)
		if len(chunk) >= 10000 {
			wg.Add(1)
			currentChunk := make([]string, len(chunk))
			copy(currentChunk, chunk)
			currentWorker := workers[workerIndex%len(workers)]
			// workerIndex++
			go sendChunk(chunkID, currentChunk, currentWorker, &wg)
			chunkID++
			chunk = []string{}
		}
	}
	log.Print("Finished reading the file")
	if len(chunk) > 0 {
		wg.Add(1)
		currentChunk := make([]string, len(chunk))
		copy(currentChunk, chunk)
		currentWorker := workers[workerIndex%len(workers)]
		go sendChunk(chunkID, currentChunk, currentWorker, &wg)
		chunkID++
	}

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

func sendChunk(id int, lines []string, workerAddr string, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, err := grpc.Dial(workerAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to dial worker %s: %v", workerAddr, err)
	}
	defer conn.Close()

	client := pb.NewMapReduceServiceClient(conn)

	logData := []byte(fmt.Sprintf("%v", joinLines(lines)))

	req := &pb.MapRequest{
		ChunkId: fmt.Sprintf("chunk-%d", id),
		LogData: logData,
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