package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/flynn/flynn/pkg/shutdown"
)

func main() {
	seed := flag.Int64("seed", time.Now().UnixNano(), "random number seed")
	peers := flag.String("peers", "127.0.0.1", "peer IPs")
	ports := flag.Int("ports", 5, "# of ports to listen on")
	clients := flag.Int("clients", 10, "# of clients to start")
	duration := flag.Duration("duration", 30*time.Second, "duration to run for")
	runServer := flag.Bool("server", true, "run server as well as clients")
	flag.Parse()
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	peerList := strings.Split(*peers, ",")
	rand.Seed(*seed)
	log.Printf("starting netspam with ports=%d clients=%d seed=%d peers=%v", *ports, *clients, *seed, peerList)

	wg := &sync.WaitGroup{}
	shutdown.BeforeExit(wg.Wait)
	stopping := make(chan struct{})
	shutdown.BeforeExit(func() { close(stopping) })

	if *runServer {
		for i := 0; i < *ports; i++ {
			l, err := net.Listen("tcp", fmt.Sprintf(":%d", i+4000))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("started listener on %s", l.Addr())

			wg.Add(1)
			shutdown.BeforeExit(func() { l.Close() })
			go server(l, wg)
		}
	}

	addrs := make([]string, 0, len(peerList)*(*ports))
	for _, p := range peerList {
		for i := 0; i < *ports; i++ {
			addrs = append(addrs, fmt.Sprintf("%s:%d", p, i+4000))
		}
	}
	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go client(addrs, wg, stopping)
	}

	<-time.After(*duration)
	shutdown.Exit()
}

const (
	maxData  = 10000000
	maxChunk = 1024
	deadline = 1 * time.Second
)

var dialer = net.Dialer{Timeout: deadline}

func server(l net.Listener, wg *sync.WaitGroup) {
	var connWg sync.WaitGroup
	for {
		conn, err := l.Accept()
		if err != nil {
			connWg.Wait()
			wg.Done()
			log.Printf("stopped listener %s", l.Addr())
			return
		}

		connWg.Add(1)
		go runConn(conn, true, &connWg)
	}
}

func client(addrs []string, wg *sync.WaitGroup, stopping chan struct{}) {
	for {
		select {
		case <-stopping:
			wg.Done()
			log.Println("stopped client")
			return
		default:
		}

		addr := addrs[rand.Intn(len(addrs))]
		conn, err := dialer.Dial("tcp", addr)
		if err != nil {
			log.Printf("%s error dialing: %s", addr, err)
			continue
		}
		runConn(conn, false, nil)
	}
}

func runConn(conn net.Conn, server bool, wg *sync.WaitGroup) {
	var connInfo string
	if server {
		connInfo = fmt.Sprintf("%s<-%s", conn.LocalAddr(), conn.RemoteAddr())
		log.Printf("%s new incoming connection", connInfo)
		defer wg.Done()
	} else {
		connInfo = fmt.Sprintf("%s->%s", conn.LocalAddr(), conn.RemoteAddr())
		log.Printf("%s new outgoing connection", connInfo)
	}
	defer func() {
		log.Printf("%s finished", connInfo)
	}()
	defer conn.Close()

	outgoing := rand.Int63n(maxData) + 1
	if err := binary.Write(conn, binary.BigEndian, outgoing); err != nil {
		log.Printf("%s error writing handshake: %s", connInfo, err)
	}
	var incoming int64
	if err := binary.Read(conn, binary.BigEndian, &incoming); err != nil {
		log.Printf("%s error reading handshake: %s", connInfo, err)
		return
	}

	log.Printf("%s successful handshake: incoming=%d outgoing=%d", connInfo, incoming, outgoing)

	buf := make([]byte, maxChunk)
	var sent, received int64
	for {
		write := func() {
			if sent < outgoing {
				chunk := rand.Int63n(maxChunk) + 1
				if sent+chunk > outgoing {
					chunk = outgoing - sent
				}
				conn.SetWriteDeadline(time.Now().Add(deadline))
				if err := binary.Write(conn, binary.BigEndian, chunk); err != nil {
					log.Printf("%s write len error sent=%d received=%d: %s", connInfo, sent, received, err)
					return
				}
				n, err := conn.Write(buf[:chunk])
				if err != nil {
					log.Printf("%s write error sent=%d received=%d size=%d: %s", connInfo, sent, received, chunk, err)
					return
				}
				sent += int64(n)
			}
		}

		read := func() {
			if received < incoming {
				var chunk int64
				conn.SetReadDeadline(time.Now().Add(deadline))
				if err := binary.Read(conn, binary.BigEndian, &chunk); err != nil {
					log.Printf("%s read len error sent=%d received=%d: %s", connInfo, sent, received, err)
					return
				}
				if n, err := io.ReadFull(conn, buf[:chunk]); err != nil {
					log.Printf("%s read error sent=%d received=%d size=%d read=%d: %s", connInfo, sent, received, chunk, n, err)
					return
				}
				received += chunk
			}
		}

		if server {
			write()
			read()
		} else {
			read()
			write()
		}

		if received >= incoming && sent >= outgoing {
			log.Printf("%s complete sent=%d received=%d", connInfo, sent, received)
			return
		}
	}
}
