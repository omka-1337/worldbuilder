// Command worldbuilder serves a library of worlds, each a directory of
// Markdown articles, and opens it in the browser.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"worldbuilder/internal/library"
	"worldbuilder/internal/server"
	"worldbuilder/web"
)

func main() {
	defaultRoot, err := library.DefaultRoot()
	if err != nil {
		log.Fatal(err)
	}
	root := flag.String("library", defaultRoot, "folder that holds the worlds, one subfolder each")
	addr := flag.String("addr", "127.0.0.1:7777", "address to listen on")
	noOpen := flag.Bool("no-open", false, "do not open the browser")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: worldbuilder [flags]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 0 {
		flag.Usage()
		os.Exit(2)
	}
	explicitAddr := false
	flag.Visit(func(f *flag.Flag) { explicitAddr = explicitAddr || f.Name == "addr" })
	if err := run(*root, *addr, explicitAddr, !*noOpen); err != nil {
		log.Fatal(err)
	}
}

func run(root, addr string, explicitAddr, open bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lib, err := library.New(ctx, root)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil && !explicitAddr {
		// The default port is taken. If it is this app serving the same
		// library, just point the browser at it; otherwise (another program,
		// or an instance suspended with Ctrl+Z) pick any free port.
		if url, ok := runningInstance(addr, lib.Root()); ok {
			log.Printf("already running at %s", url)
			if open {
				openBrowser(url)
			}
			return nil
		}
		log.Printf("%s is busy, using another port", addr)
		ln, err = listenNear(addr)
	}
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: server.New(lib, web.Dist()), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	_, port, _ := net.SplitHostPort(ln.Addr().String())
	url := "http://localhost:" + port
	log.Printf("worlds in %s, serving at %s", root, url)
	if open {
		openBrowser(url)
	}

	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

// listenNear tries the ten ports after addr's, then any free one. Nearby
// ports keep the browser's per-origin storage (theme, open groups) more
// often than a random one would.
func listenNear(addr string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if n, convErr := strconv.Atoi(port); err == nil && convErr == nil {
		for p := n + 1; p <= n+10; p++ {
			if ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(p))); err == nil {
				return ln, nil
			}
		}
	}
	return net.Listen("tcp", "127.0.0.1:0")
}

// runningInstance reports whether addr is a responsive worldbuilder serving
// the library at root, and its URL.
func runningInstance(addr, root string) (string, bool) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", false
	}
	url := "http://localhost:" + port
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(url + "/api/worlds")
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	var info struct {
		Root string `json:"root"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&info) != nil {
		return "", false
	}
	return url, sameDir(info.Root, root)
}

func sameDir(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("open browser: %v", err)
		return
	}
	go cmd.Wait()
}
