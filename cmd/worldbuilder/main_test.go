package main

import (
	"context"
	"net"
	"net/http"
	"testing"

	"worldbuilder/internal/library"
	"worldbuilder/internal/server"
	"worldbuilder/web"
)

func TestRunningInstance(t *testing.T) {
	root := t.TempDir()
	lib, err := library.New(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: server.New(lib, web.Dist())}
	go srv.Serve(ln)
	defer srv.Close()
	addr := ln.Addr().String()

	if _, ok := runningInstance(addr, root); !ok {
		t.Error("same library not recognized")
	}
	if _, ok := runningInstance(addr, t.TempDir()); ok {
		t.Error("a different library must not be reused")
	}

	// A port where nothing answers HTTP, like a process stopped with Ctrl+Z.
	silent, _ := net.Listen("tcp", "127.0.0.1:0")
	defer silent.Close()
	if _, ok := runningInstance(silent.Addr().String(), root); ok {
		t.Error("an unresponsive port taken for an instance")
	}
}
