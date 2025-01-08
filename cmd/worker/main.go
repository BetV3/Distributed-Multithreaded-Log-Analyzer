package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"regexp"

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
	// Split the log data by new line
	lines := bytes.Split(req.LogData, []byte("\n"))
	// Create a map to store the counts of each status code
	counts := make(map[string]int64)
	// Iterate over each line and extract the status code
	for _, lineBytes := range lines {
		if len(lineBytes) == 0 {
			continue
		}
		// Convert the line to a string
		line := string(lineBytes)
		// Define a regex to extract the fields
		logRegex := regexp.MustCompile(`(?P<IP>\S+) \S+ \S+ \[(?P<Date>[^\]]+)] "(?P<Request>[^"]*)" (?P<StatusCode>\d{3}) (?P<Size>\d+) "(?P<Referrer>[^"]*)" "(?P<UserAgent>[^"]*)"`)
		// Find the matches
		matches := logRegex.FindStringSubmatch(line)
		if matches == nil {
			fmt.Println("No match found")
			continue
		}
		// Extract the fields
		fields := []string {
			matches[1], // IP address
			matches[2], // Date
			matches[3], // Request
			matches[4], // Status code
			matches[5], // Response size
			matches[6], // Referrer
			matches[7], // User agent
		}
		statusCode := fields[3]
		counts[statusCode]++
	}

	// Prepare the partial results
	var partialResults []*pb.PartialResult
	for k, v := range counts {
		partialResults = append(partialResults, &pb.PartialResult{
			Key: k,
			Count: v,
		})
	}
	// Return the partial results
	return &pb.MapResponse{
		PartialResults: partialResults,
	}, nil
}


func main() {
	// Create a listener on port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	// Create a grpc options to allow large messages (64MB)
	serverOptions := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(1024 * 1024 * 64),
		grpc.MaxSendMsgSize(1024 * 1024 * 64),
	}
	// Create a new gRPC server with the options
	grpcServer := grpc.NewServer(serverOptions...)
	pb.RegisterMapReduceServiceServer(grpcServer, &workerServer{})

	//Will implement server reflection for debugging
	//reflection.Register(grpcServer)
	log.Printf("[WORKER] Starting gRPC server on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}