package httpserver

import (
	"context"
	"io"
	"net"
	"net/http"
)

type ListenCloser interface {
	Listen()
	io.Closer
}

type logger interface {
	Printf(string, ...interface{})
}
type monitor interface {
	PanicRecovered(request *http.Request, err interface{})
}

type httpServer interface {
	Serve(listener net.Listener) error
	Shutdown(ctx context.Context) error
}

type listenConfig interface {
	Listen(ctx context.Context, network, address string) (net.Listener, error)
}
