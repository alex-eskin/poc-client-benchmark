package main

import (
	"fmt"
	"sync/atomic"

	"net/http"
	"os"
	"runtime/pprof"
	"testing"
	"time"
)

func BenchmarkAddEndpoint(b *testing.B) {
	url := "http://localhost:8080/add?a=1&b=2"
	totalRequests := 100 // Total number of requests to send
	b.N = totalRequests  // Set the number of iterations for the benchmark

	b.ReportAllocs()
	cpuProfile, _ := os.Create(fmt.Sprintf("cpu_profile_%d.prof", totalRequests))
	defer cpuProfile.Close()
	pprof.StartCPUProfile(cpuProfile)
	defer pprof.StopCPUProfile()

	start := time.Now()

	b.ResetTimer()
	counter := atomic.Int32{}
	fmt.Printf("Running benchmark with %d requests to %s\n", totalRequests, url)
	client := &http.Client{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counterValue := counter.Add(1)

			resp, err := client.Get(url)

			if err == nil && resp.Body != nil {
				//body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				//fmt.Printf("Counter: %d | Args: a=1, b=2 | Response: %s\n", counterValue, string(body))
			} else {
				fmt.Printf("Counter: %d | Args: a=1, b=2 | Error: %v\n", counterValue, err)
			}
		}
	})
	b.StopTimer()
	elapsed := time.Since(start)
	latency := float64(elapsed.Microseconds()) / float64(totalRequests)
	throughput := float64(totalRequests) / elapsed.Seconds()

	fmt.Printf("Requests: %d | Latency: %.2f µs/op | Throughput: %.2f ops/sec\n",
		totalRequests, latency, throughput)
	fmt.Printf("CPU profile written to: cpu_profile_%d.prof\n", totalRequests)

}
