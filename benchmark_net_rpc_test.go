package main

import (
	"fmt"
	"math/rand"
	"net/rpc"
	"os"
	"runtime/pprof"
	"sync/atomic"
	"testing"
	"time"
)

type AddArgs struct {
	A, B int
}

type AddReply struct {
	Result int
}

func BenchmarkAddNetRPC(b *testing.B) {
	address := "localhost:8081"
	totalRequests := 10000
	b.N = totalRequests

	b.ReportAllocs()
	cpuProfile, _ := os.Create(fmt.Sprintf("cpu_profile_net_rpc_%d.prof", totalRequests))
	defer cpuProfile.Close()
	pprof.StartCPUProfile(cpuProfile)
	defer pprof.StopCPUProfile()

	start := time.Now()
	b.ResetTimer()
	counter := atomic.Int32{}
	fmt.Printf("Running net/rpc benchmark with %d requests to %s\n", totalRequests, address)

	client, err := rpc.Dial("tcp", address)
	if err != nil {
		fmt.Printf("Failed to dial: %v\n", err)
		return
	}
	defer client.Close()

	client.Call("AddService.Add", &AddArgs{A: 1, B: 2}, &AddReply{}) // warmup call

	b.RunParallel(func(pb *testing.PB) {

		for pb.Next() {
			counterValue := counter.Add(1)
			args := &AddArgs{A: rand.Intn(1000), B: rand.Intn(1000)}
			var reply AddReply
			err := client.Call("AddService.Add", args, &reply)
			if err != nil {
				fmt.Printf("Counter: %d | Args: a=1, b=2 | Error: %v\n", counterValue, err)
			} else if reply.Result != args.A+args.B {
				fmt.Printf("Counter: %d | Args: a=%d, b=%d | Wrong result: got %d, want %d\n", counterValue, args.A, args.B, reply.Result, args.A+args.B)
			}
			// Optionally print reply.Sum if needed
		}
	})

	b.StopTimer()
	elapsed := time.Since(start)
	latency := float64(elapsed.Microseconds()) / float64(totalRequests)
	throughput := float64(totalRequests) / elapsed.Seconds()

	fmt.Printf("Requests: %d | Latency: %.2f µs/op | Throughput: %.2f ops/sec\n",
		totalRequests, latency, throughput)
	fmt.Printf("CPU profile written to: cpu_profile_net_rpc_%d.prof\n", totalRequests)
}
