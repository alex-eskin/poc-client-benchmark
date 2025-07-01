package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"runtime/pprof"
	"testing"
	"time"

	"context"

	pbc "poc1_client_benchmark/add"
)

type jwtPerRPCCredentials struct {
	token string
}

func (j *jwtPerRPCCredentials) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + j.token,
	}, nil
}

func (j *jwtPerRPCCredentials) RequireTransportSecurity() bool {
	return false // Set to true if using TLS
}

type Post struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

func getCognitoJWTToken() string {
	posturl := "https://us-east-1a5zky0lb1.auth.us-east-1.amazoncognito.com/oauth2/token"

	clientSecret := "1que206glum6i4n0a9upgdurrfdoev2msk6bp1jdu83ogo29ne9u"
	body := []byte(`grant_type=client_credentials&client_id=7p26dggomphc1ho0i55u5s3h65&client_secret=` + clientSecret + `&scope=default-m2m-resource-server-sgrxu7/read`)

	r, err := http.NewRequest("POST", posturl, bytes.NewBuffer(body))

	if err != nil {
		panic(err)
	}

	r.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		panic(err)
	}

	defer res.Body.Close()

	post := &Post{}
	//fmt.Println("Response Status:", res.Status)
	//fmt.Println("Response Body:", res.Body)
	derr := json.NewDecoder(res.Body).Decode(post)
	if derr != nil {
		panic(derr)
	}

	//fmt.Println("AccessToken:", post.AccessToken)
	//fmt.Println("ExpiresIn:", post.ExpiresIn)
	//fmt.Println("TokenType:", post.TokenType)

	return post.AccessToken
}

func BenchmarkAddGrpcEndpoint(b *testing.B) {
	totalRequests := 10 // Total number of requests to send
	b.N = totalRequests // Set the number of iterations for the benchmark
	url := "localhost:8082"

	jwtToken := getCognitoJWTToken()

	b.ReportAllocs()
	cpuProfile, _ := os.Create(fmt.Sprintf("cpu_profile_grpc_%d.prof", totalRequests))
	defer cpuProfile.Close()
	pprof.StartCPUProfile(cpuProfile)
	defer pprof.StopCPUProfile()

	counter := atomic.Int32{}
	fmt.Printf("Running benchmark with %d requests to %s\n", totalRequests, url)
	//conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()),
	//	grpc.WithPerRPCCredentials(&jwtPerRPCCredentials{token: jwtToken}))

	err, creds := getCredsWithCert()

	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := grpc.NewClient(
		url,
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(&jwtPerRPCCredentials{token: jwtToken}),
	)

	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	client := pbc.NewAddServiceClient(conn)

	client.Add(context.Background(), &pbc.AddRequest{A: 1, B: 2}) // warmup call

	start := time.Now()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			counterValue := counter.Add(1)

			aVal := int32(rand.Intn(1000)) // random int between 0 and 999
			bVal := int32(rand.Intn(1000))

			resp, err := client.Add(context.Background(), &pbc.AddRequest{A: aVal, B: bVal})

			if err != nil {
				fmt.Printf("Counter: %d | Args: a=1, b=2 | Error: %v\n", counterValue, err)
			} else if resp.Result != aVal+bVal {
				fmt.Printf("Counter: %d | Args: a=%d, b=%d | Wrong result: got %d, want %d\n", counterValue, aVal, bVal, resp.Result, aVal+bVal)
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

func getCredsWithCert() (error, credentials.TransportCredentials) {
	// Load client cert and key
	clientCert, err := tls.LoadX509KeyPair("./client.crt", "./client.key")
	if err != nil {
		return err, nil
	}

	// Load server cert
	serverCert, err := os.ReadFile("./server.crt") // or your CA cert
	if err != nil {
		return err, nil
	}
	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(serverCert); !ok {
		return fmt.Errorf("failed to append CA cert"), nil
	}

	// Create tls.Config for mTLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      certPool,
		// Optionally, set ServerName if needed
		// ServerName: "localhost",
	}

	creds := credentials.NewTLS(tlsConfig)
	return nil, creds

	//certPool := x509.NewCertPool()
	//caCert, err := os.ReadFile("./server.crt") // Replace with your CA cert path
	//if err != nil {
	//	panic(err)
	//}
	//if ok := certPool.AppendCertsFromPEM(caCert); !ok {
	//	panic("failed to append CA cert")
	//}
	//
	//creds := credentials.NewClientTLSFromCert(certPool, "")
	//return err, creds
}
