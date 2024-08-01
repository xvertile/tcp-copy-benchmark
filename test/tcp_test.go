package test

import (
	"bufio"
	"crypto/rand"
	"golang.org/x/sys/unix"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"runtime/pprof"
	"sync"
	"syscall"
	"testing"
	"time"
)

const (
	address    = "localhost:12345"
	numClients = 5000
	bufferSize = 1024 * 1024
)

var (
	message       = generateRandomString(10240) // Generate a 1MB random string
	messageLength = len(message)
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

func generateRandomString(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, size)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			log.Fatal(err)
		}
		b[i] = charset[num.Int64()]
	}
	return string(b)
}
func startServer() {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
	defer listener.Close()
	log.Println("Server started")

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, bufferSize)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading from connection: %v", err)
			}
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Printf("Error writing to connection: %v", err)
			return
		}
	}
}

func init() {
	go startServer()
	time.Sleep(time.Second)
}
func BenchmarkIoPipe(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_IoPipe.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			pr, pw := io.Pipe()
			defer pr.Close()
			defer pw.Close()

			go func() {
				for n := 0; n < b.N; n++ {
					_, err := pw.Write([]byte(message))
					if err != nil {
						b.Fatalf("Error writing to pipe: %v", err)
					}
				}
			}()

			for n := 0; n < b.N; n++ {
				resp := make([]byte, messageLength)
				_, err := io.ReadFull(pr, resp)
				if err != nil {
					b.Fatalf("Error reading from pipe: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
func BenchmarkIoCopy(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_IoCopy.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			for n := 0; n < b.N; n++ {
				_, err := conn.Write([]byte(message))
				if err != nil {
					b.Fatalf("Error writing to connection: %v", err)
				}

				resp := make([]byte, messageLength)
				_, err = io.ReadFull(conn, resp)
				if err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkIoCopyBuffer(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_IoCopyBuffer.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			for n := 0; n < b.N; n++ {
				_, err := conn.Write([]byte(message))
				if err != nil {
					b.Fatalf("Error writing to connection: %v", err)
				}

				resp := make([]byte, messageLength)
				buf := bufferPool.Get().([]byte)
				defer bufferPool.Put(buf)
				_, err = io.ReadFull(conn, resp)
				if err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
func BenchmarkSyscall(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_Syscall.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
			if err != nil {
				b.Fatalf("Error creating socket: %v", err)
			}
			defer syscall.Close(fd)

			sa := &syscall.SockaddrInet4{Port: 12345}
			copy(sa.Addr[:], net.ParseIP("127.0.0.1").To4())

			err = syscall.Connect(fd, sa)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}

			for n := 0; n < b.N; n++ {
				_, err := syscall.Write(fd, []byte(message))
				if err != nil {
					b.Fatalf("Error writing to socket: %v", err)
				}

				resp := make([]byte, messageLength)
				_, err = syscall.Read(fd, resp)
				if err != nil {
					b.Fatalf("Error reading from socket: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkIoCopyDirect(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_IoCopyDirect.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			for n := 0; n < b.N; n++ {
				_, err := conn.Write([]byte(message))
				if err != nil {
					b.Fatalf("Error writing to connection: %v", err)
				}

				_, err = io.CopyN(io.Discard, conn, int64(messageLength))
				if err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
func BenchmarkUnixSyscall(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_UnixSyscall.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			fd, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP)
			if err != nil {
				b.Fatalf("Error creating socket: %v", err)
			}
			defer unix.Close(fd)

			sa := &unix.SockaddrInet4{Port: 12345}
			copy(sa.Addr[:], net.ParseIP("127.0.0.1").To4())

			err = unix.Connect(fd, sa)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}

			for n := 0; n < b.N; n++ {
				_, err := unix.Write(fd, []byte(message))
				if err != nil {
					b.Fatalf("Error writing to socket: %v", err)
				}

				resp := make([]byte, messageLength)
				_, err = unix.Read(fd, resp)
				if err != nil {
					b.Fatalf("Error reading from socket: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkBufio(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_Bufio.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)
			writer := bufio.NewWriter(conn)

			for n := 0; n < b.N; n++ {
				_, err := writer.WriteString(message)
				if err != nil {
					b.Fatalf("Error writing to connection: %v", err)
				}
				err = writer.Flush()
				if err != nil {
					b.Fatalf("Error flushing writer: %v", err)
				}

				resp := make([]byte, messageLength)
				_, err = io.ReadFull(reader, resp)
				if err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func BenchmarkSplice(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_Splice.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			connFile, err := conn.(*net.TCPConn).File()
			if err != nil {
				b.Fatalf("Error getting connection file: %v", err)
			}
			defer connFile.Close()

			connFd := int(connFile.Fd())

			pr, pw, err := os.Pipe()
			if err != nil {
				b.Fatalf("Error creating pipe: %v", err)
			}
			defer pr.Close()
			defer pw.Close()

			pipeFd := int(pr.Fd())

			for n := 0; n < b.N; n++ {
				_, err := pw.Write([]byte(message))
				if err != nil {
					b.Fatalf("Error writing to pipe: %v", err)
				}

				_, err = unix.Splice(pipeFd, nil, connFd, nil, messageLength, unix.SPLICE_F_MOVE)
				if err != nil {
					b.Fatalf("Error splicing: %v", err)
				}

				resp := make([]byte, messageLength)
				_, err = io.ReadFull(conn, resp)
				if err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
func BenchmarkSendfile(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_Sendfile.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			connFile, err := conn.(*net.TCPConn).File()
			if err != nil {
				b.Fatalf("Error getting connection file: %v", err)
			}
			defer connFile.Close()

			connFd := int(connFile.Fd())

			tempFile, err := os.CreateTemp("", "sendfile")
			if err != nil {
				b.Fatalf("Error creating temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())
			defer tempFile.Close()

			for n := 0; n < b.N; n++ {
				_, err := tempFile.Write([]byte(message))
				if err != nil {
					b.Fatalf("Error writing to temp file: %v", err)
				}
				if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
					b.Fatalf("Error seeking temp file: %v", err)
				}

				tempFd := int(tempFile.Fd())

				if _, err := unix.Sendfile(connFd, tempFd, nil, messageLength); err != nil {
					b.Fatalf("Error with sendfile: %v", err)
				}

				resp := make([]byte, messageLength)
				if _, err := io.ReadFull(conn, resp); err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
func BenchmarkReadvWritev(b *testing.B) {
	cpuProfile, err := os.Create("results/cpu_profile_ReadvWritev.prof")
	if err != nil {
		log.Fatalf("could not create CPU profile: %v", err)
	}
	defer cpuProfile.Close()

	if err := pprof.StartCPUProfile(cpuProfile); err != nil {
		log.Fatalf("could not start CPU profile: %v", err)
	}
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", address)
			if err != nil {
				b.Fatalf("Error connecting to server: %v", err)
			}
			defer conn.Close()

			connFile, err := conn.(*net.TCPConn).File()
			if err != nil {
				b.Fatalf("Error getting connection file: %v", err)
			}
			defer connFile.Close()

			connFd := int(connFile.Fd())

			for n := 0; n < b.N; n++ {
				iov := [][]byte{[]byte(message)}
				if _, err := unix.Writev(connFd, iov); err != nil {
					b.Fatalf("Error writing to connection: %v", err)
				}

				resp := make([]byte, messageLength)
				iov = [][]byte{resp}
				if _, err := unix.Readv(connFd, iov); err != nil {
					b.Fatalf("Error reading from connection: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}
