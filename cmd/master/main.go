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

// For now, we will not only hardcode and use our local machine as the worker
var workers = []string{
	"192.168.0.238:50051", // bravo worker 1
	"192.168.0.241:50051", // bravo worker 2
	"192.168.0.207:50051", // bravo worker 3
	"192.168.0.206:50051", // alpha worker 4
	"192.168.0.222:50051", // alpha worker 5
	"192.168.0.216:50051", // bravo worker 6
	"192.168.0.224:50051", // bravo worker 7
	"192.168.0.231:50051", // delta worker 8
	"192.168.0.202:50051", // delta worker 9
	"192.168.0.227:50051", // charlie worker 10
	"192.168.0.221:50051", // charlie worker 11
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
	var (
		allPartialResults []*pb.PartialResult
		mu                sync.Mutex
	)

	var wg sync.WaitGroup
	chunkID := 0
	workerIndex := 0
	buffer := make([]byte, chunkSize)
	leftover := make([]byte, 0)
	for {
		n, err := file.Read(buffer)
		if n == 0 {
			break
		}
		wg.Add(1)
		data := append(leftover, buffer[:n]...)
		lastNewline := bytes.LastIndexByte(data, '\n')
		if lastNewline == -1 {
			lastNewline = len(data)
		}
		chunk := data[:lastNewline]
		leftover = data[lastNewline+1:]
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


func sendChunk(id int, lines []byte, workerAddr string, wg *sync.WaitGroup, allPartialResults *[]*pb.PartialResult, mu *sync.Mutex) {
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
	mu.Lock()
	*allPartialResults = append(*allPartialResults, resp.PartialResults...)
	mu.Unlock()
	//log.Printf("Chunk %d processed by worker %s, with %d partial results", id, workerAddr, len(resp.PartialResults))
	//log.Printf("Partial results: %v", localReduce(resp.PartialResults))
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