package main

import (
	"bytes"
	"context"
	"log"
	"net"
	"strings"

	pb "gitlab.betv3.xyz/BetV3/distributed-key-value-store/internal/grpc"
	"google.golang.org/grpc"
)

// workerServer implements the pb.MapReduceServiceServer
type workerServer struct {
	pb.UnimplementedMapReduceServiceServer
}

// ProcessMap handles the Map phase of the job
func (s *workerServer) ProcessMap(ctx context.Context, req *pb.MapRequest) (*pb.MapResponse, error){
	log.Printf("[WORKER] Recieved Map request for chunk %s", req.ChunkId)

	// 1. Parse the req.LogData, do the "map" logic (counting status codes and stuff like that)
	lines := bytes.Split(req.LogData, []byte("\n"))

	counts := make(map[string]int64)
	// example log line: 
	// "in24.inetnebr.com - - [01/Aug/1995:00:00:01 -0400] "GET /shuttle/missions/sts-68/news/sts-68-mcc-05.txt HTTP/1.0" 200 1839

	for _, lineBytes := range lines {
		if len(lineBytes) == 0 {
			continue
		}
		line := string(lineBytes)
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		statusCode := parts[8]
		counts[statusCode]++
	}

	// 2 Prepare the partial results
	var partialResults []*pb.PartialResult
	for k, v := range counts {
		partialResults = append(partialResults, &pb.PartialResult{
			Key: k,
			Count: v,
		})
	}
	return &pb.MapResponse{
		PartialResults: partialResults,
	}, nil
}

// ProcessReduce handles the Reduce phase of the ob (when master does not implement)
func (s *workerServer) ProcessReduce(ctx context.Context, req *pb.ReduceRequest) (*pb.ReduceResponse, error) {
	log.Printf("[WORKER] received ReduceRequest with %d partial results", len(req.PartialResults))

	//Merge partial results into aggregated results
	aggregated := doReduceOperation(req.PartialResults)

	return &pb.ReduceResponse{
		Results: aggregated,
	}, nil
}

// helper function to parse log data and return partial data
func doMapOperation(logData []byte) []*pb.PartialResult {
	// For now, we will just return a dummy result
	return []*pb.PartialResult{
		{
			Key:   "200",
			Count: 42,
		},
		{
			Key: "404",
			Count: 3,
		},
	}
}
// helper function to merge partial results into aggregated results
func doReduceOperation(partialResults []*pb.PartialResult) []*pb.AggregatedResult {
	counts := make(map[string]int64)
	for _, pr := range partialResults {
		counts[pr.Key] += pr.Count
	}

	var result []*pb.AggregatedResult
	for k, v := range counts {
		result = append(result, &pb.AggregatedResult{Key: k, TotalCount: v})
	}
	return result
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(1024 * 1024 * 64),
		grpc.MaxSendMsgSize(1024 * 1024 * 64),
	}
	grpcServer := grpc.NewServer(serverOptions...)
	pb.RegisterMapReduceServiceServer(grpcServer, &workerServer{})

	//Will implement server reflection for debugging
	//reflection.Register(grpcServer)
	log.Printf("[WORKER] Starting gRPC server on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}