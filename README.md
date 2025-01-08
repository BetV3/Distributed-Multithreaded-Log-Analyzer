**Distributed Multithreaded Log Analyzer**

A high-performance log analysis tool built in Go that utilizes a Master-Worker architecture to process large-scale web server logs. The system reads and distributes a 3.3GB log file containing over 10 million lines across multiple worker nodes, efficiently extracting HTTP status codes. Leveraging Go’s concurrency primitives (goroutines, channels) and distributed coordination, the analyzer scales nearly linearly—reducing processing time from 36 seconds on a single worker to 6 seconds using 8–11 workers. 

### Features
- **Scalable Architecture**: Master distributes tasks to worker nodes for parallel processing.
- **Efficient Concurrency**: Utilizes Go's lightweight goroutines and channels to handle high-volume I/O and CPU-bound parsing.
- **Performance Optimization**: Achieves significant speedup and resource efficiency through careful profiling, buffer management, and dynamic scaling.
- **Robust Processing**: Designed to handle large files (3.3GB, 10 million lines) reliably across distributed systems.

### Getting Started
1. Clone the repository:
   ```bash
   git clone https://github.com:BetV3/Distributed-Multithreaded-Log-Analyzer.git
   cd distributed-multithreaded-log-analyzer
   ```
2. Build the Master and Worker binaries:
   ```bash
   go build -o master ./cmd/master
   go build -o worker ./cmd/worker
   ```
3. Follow the README for configuration, deployment instructions, and usage details.

### Technologies Used
- **Language**: Go
- **Architecture**: Master-Worker Distributed Processing
- **Networking**: gRPC for inter-node communication
- **Concurrency**: Goroutines, Channels
- **Deployment**: Docker, Kubernetes (optional)

Contributions, improvements, and feedback are welcome!
