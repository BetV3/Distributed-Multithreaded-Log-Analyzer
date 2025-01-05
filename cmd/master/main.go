package main

import (
	"context"
	"log"
	"net"

	pb "gitlab.betv3.xyz/BetV3/distributed-key-value-store/internal/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// masterServer implements the MapReduceServiceServer interface
// In a typical mini-MapReduce, the workers might implement these methods
// But if your design calls for the master to also handle gRPC requests, here is an example
type masterServer struct {
	pb.UnimplementedMapReduceServiceServer
}

// ProcessReduce is called when a client (possibly a worker or external client)
// Sends a MapRequest to this Master Service
func (m *masterServer) ProcessMap(ctx context.Context, req *pb.MapRequest) (*pb.MapResponse, error) {
	log.Println(("[MASTER] Received ProcessMap request for chunk: %s"), req.ChunkId)

	// -- Example: Just log or mock a response for now -- 
	// eventually, might distribute tasks to other components
	// or store partial resultss in a map, etc.

	return &pb.MapResponse{
		PartialResults: []*pb.PartialResult{
			{
				Key:	"dummy_key",
				Count: 1,
			},
		},
	}, nil
}

// ProcessReduce merges partial results. In the master-as-server scenario,
// the caller (worker or specialized reducer node) might send partial results,
// and the master aggregates them.
func (m *masterServer) ProcessReduce(ctx context.Context, req *pb.ReduceRequest) (*pb.ReduceResponse, error) {
	log.Println(("[MASTER] Received ProcessReduce request"))

	// -- Example: Summation of counts form all partial results -- 
	aggregatedCounts := map[string]int64{}

	for _, pr := range req.PartialResults {
		aggregatedCounts[pr.Key] += pr.Count
	}

	var results []*pb.AggregatedResult
	for k, v := range aggregatedCounts {
		results = append(results, &pb.AggregatedResult{
			Key: k,
			TotalCount: v,
		})
	}

	return &pb.ReduceResponse{
		Results: results,
	}, nil
}

func main() {
	// Create Listen on TCP port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	// Create a gRPC server
	grpcServer := grpc.NewServer()

	// Register our masterServer as the gRPC handler
	pb.RegisterMapReduceServiceServer(grpcServer, &masterServer{})

	log.Println("[MASTER] server started on port 50051")

	// start serving requests
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}

}