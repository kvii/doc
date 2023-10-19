// Doc provide a command line tool to serve your local files.
//
// Some programming language can only generate
// package doc, but do not provide http access,
// such as rust, dart.
// It is useful when you want to view them immediately,
// instead of copy doc files to somewhere like nginx.
package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/kvii/handler"
)

var (
	addr string // serve address
	dir  string // root path
)

func init() {
	flag.StringVar(&addr, "addr", "localhost:9090", "serve address")
	flag.StringVar(&dir, "dir", ".", "root path")
}

func main() {
	flag.Parse()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	ctx := context.Background()
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	s := http.Server{
		Handler: handler.VueServer(http.Dir(dir)),
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("Serve at http://%s\n", l.Addr())
		err = s.Serve(l)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		log.Println("Closing server...")
		s.Close()
	}()

	wg.Wait()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
