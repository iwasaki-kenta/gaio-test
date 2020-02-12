package main

import (
	"errors"
	"github.com/xtaci/gaio"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

func server(w *gaio.Watcher) {
	disconnected := make(map[string]struct{})

	for {
		results, err := w.WaitIO()
		if err != nil {
			log.Println("got an error:", err)
			return
		}

		for _, res := range results {
			addr := res.Conn.LocalAddr().String()

			if _, disconnected := disconnected[addr]; disconnected {
				continue
			}

			if res.Error == nil {
				var err error

				switch res.Operation {
				case gaio.OpRead: // read completion event
					err = w.WriteTimeout(nil, res.Conn, res.Buffer[:res.Size], time.Now().Add(3*time.Second))
				case gaio.OpWrite: // write completion event
					err = w.ReadTimeout(nil, res.Conn, res.Buffer[:cap(res.Buffer)], time.Now().Add(3*time.Second))
				}

				if errors.Is(err, gaio.ErrWatcherClosed) {
					break
				}

				continue
			}

			if errors.Is(res.Error, io.EOF) {
				disconnected[addr] = struct{}{}

				log.Println("conn closed", res.Conn.RemoteAddr())

				continue
			}

			if errors.Is(res.Error, gaio.ErrDeadline) {
				disconnected[addr] = struct{}{}

				if err := w.Free(res.Conn); err != nil {
					break
				}

				log.Println("conn timed out", res.Conn.RemoteAddr(), res.Operation)

				continue
			}
		}

		// optimized to memclr instruction

		for addr := range disconnected {
			delete(disconnected, addr)
		}
	}
}

func main() {
	w, err := gaio.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer w.Close()

	go server(w)

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("server listening on", listener.Addr())

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Println(err)
				return
			}

			log.Println("conn opened", conn.RemoteAddr())

			if err := w.ReadTimeout(nil, conn, make([]byte, 4), time.Now().Add(3*time.Second)); err != nil {
				log.Println(err)
				return
			}
		}
	}()

	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	<-s
}
