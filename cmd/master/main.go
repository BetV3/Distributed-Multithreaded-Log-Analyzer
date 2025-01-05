package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	pb "gitlab.betv3.xyz/BetV3/distributed-key-value-store/internal/grpc"
	"google.golang.org/grpc"
)

// For now, we will not only hardcode and use our local machine as the worker
var workers = []string{
	"localhost:50051",
}

func main() {
	log.Println("[MASTER] Starting master server...")
	// 1. Split the file into chunks
	// For example, we might read a large log file in 1MB increments or parse line by line.
	// chunkData, err := splitFile("path/to/file.log")
	chunkData := [][]byte{
		[]byte("Log chunk #1"),
		[]byte("Log chunk #2"),
	}

	// 2. Distribute chunks to workers
	var allPartialResults []*pb.PartialResult
	var mu sync.Mutex // to safely append to allPartialResults

	var wg sync.WaitGroup
	wg.Add(len(chunkData))

	for i, chunk := range chunkData {
		go func(chunkID int, data []byte) {
			defer wg.Done()
			// pick a worker randomly (could be based on round robin)
			workerAddr := workers[chunkID%len(workers)]

			// connect to worker
			conn, err := grpc.Dial(workerAddr, grpc.WithInsecure())
			if err != nil {
				log.Printf("[MASTER] Failed to dial worker at %s: %v", workerAddr, err)
				return
			}
			defer conn.Close()

			client := pb.NewMapReduceServiceClient(conn)

			req := &pb.MapRequest{
				ChunkId: fmt.Sprintf("chunk-%d", chunkID),
				LogData: data,
			}

			// Call ProcessMap on the worker
			resp, err := client.ProcessMap(context.Background(), req)
			if err != nil {
				log.Printf("[MASTER] Error calling ProcessMap on worker %s: %v", workerAddr, err)
				return
			}

			// Lock and append results
			mu.Lock()
			allPartialResults = append(allPartialResults, resp.PartialResults...)
			mu.Unlock()
		}(i, chunk)
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

func localReduce(partials []*pb.PartialResult) map[string]int64 {
	counts := make(map[string]int64)
	for _, pr := range partials {
		counts[pr.Key] += pr.Count
	}
	return counts
}