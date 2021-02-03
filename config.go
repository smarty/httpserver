package httpserver

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"syscall"
	"time"
)

type configuration struct {
	Context                  context.Context
	Handler                  http.Handler
	MaxRequestHeaderSize     int
	ReadRequestTimeout       time.Duration
	ReadRequestHeaderTimeout time.Duration
	WriteResponseTimeout     time.Duration
	IdleConnectionTimeout    time.Duration
	ShutdownTimeout          time.Duration
	ListenAddress            string
	SocketConfig             listenConfig
	TLSConfig                *tls.Config
	HandlePanic              bool
	Monitor                  monitor
	Logger                   logger
	HTTPServer               httpServer
}

func New(options ...option) ListenCloser {
	var config configuration
	Options.apply(options...)(&config)
	return newServer(config)
}

var Options singleton

type singleton struct{}
type option func(*configuration)

func (singleton) Context(value context.Context) option {
	return func(this *configuration) { this.Context = value }
}
func (singleton) ListenAddress(value string) option {
	return func(this *configuration) { this.ListenAddress = value }
}
func (singleton) TLSConfig(value *tls.Config) option {
	return func(this *configuration) { this.TLSConfig = value }
}
func (singleton) Handler(value http.Handler) option {
	return func(this *configuration) { this.Handler = value }
}
func (singleton) HandlePanic(value bool) option {
	return func(this *configuration) { this.HandlePanic = value }
}
func (singleton) HTTPServer(value httpServer) option {
	return func(this *configuration) { this.HTTPServer = value }
}
func (singleton) MaxRequestHeaderSize(value int) option {
	return func(this *configuration) { this.MaxRequestHeaderSize = value }
}
func (singleton) ReadRequestTimeout(value time.Duration) option {
	return func(this *configuration) { this.ReadRequestTimeout = value }
}
func (singleton) ReadRequestHeaderTimeout(value time.Duration) option {
	return func(this *configuration) { this.ReadRequestHeaderTimeout = value }
}
func (singleton) WriteResponseTimeout(value time.Duration) option {
	return func(this *configuration) { this.WriteResponseTimeout = value }
}
func (singleton) IdleConnectionTimeout(value time.Duration) option {
	return func(this *configuration) { this.IdleConnectionTimeout = value }
}
func (singleton) ShutdownTimeout(value time.Duration) option {
	return func(this *configuration) { this.ShutdownTimeout = value }
}
func (singleton) SocketConfig(value listenConfig) option {
	return func(this *configuration) { this.SocketConfig = value }
}
func (singleton) Monitor(value monitor) option {
	return func(this *configuration) { this.Monitor = value }
}
func (singleton) Logger(value logger) option {
	return func(this *configuration) { this.Logger = value }
}

func (singleton) apply(options ...option) option {
	return func(this *configuration) {
		for _, option := range Options.defaults(options...) {
			option(this)
		}

		if this.HandlePanic {
			this.Handler = newRecoveryHandler(this.Handler, this.Monitor, this.Logger)
		}

		if this.HTTPServer == nil {
			this.HTTPServer = &http.Server{
				Addr:              this.ListenAddress,
				Handler:           this.Handler,
				MaxHeaderBytes:    this.MaxRequestHeaderSize,
				ReadTimeout:       this.ReadRequestTimeout,
				ReadHeaderTimeout: this.ReadRequestHeaderTimeout,
				WriteTimeout:      this.WriteResponseTimeout,
				IdleTimeout:       this.IdleConnectionTimeout,
				BaseContext:       func(net.Listener) context.Context { return this.Context },
			}
		}
	}
}
func (singleton) defaults(options ...option) []option {
	var defaultSocketConfig = &net.ListenConfig{Control: func(_, _ string, conn syscall.RawConn) error {
		return conn.Control(func(descriptor uintptr) {
			_ = syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, socketReusePort, 1)
		})
	}}

	return append([]option{
		Options.ListenAddress(":http"),
		Options.TLSConfig(nil),
		Options.MaxRequestHeaderSize(1024 * 2),
		Options.ReadRequestTimeout(time.Second * 5),
		Options.ReadRequestHeaderTimeout(time.Second),
		Options.WriteResponseTimeout(time.Second * 60),
		Options.IdleConnectionTimeout(time.Second * 30),
		Options.ShutdownTimeout(time.Second * 5),
		Options.HandlePanic(true),
		Options.Context(context.Background()),
		Options.Handler(nop{}),
		Options.Monitor(nop{}),
		Options.Logger(log.New(log.Writer(), log.Prefix(), log.Flags())),
		Options.SocketConfig(defaultSocketConfig),
	}, options...)
}

type nop struct{}

func (nop) Printf(_ string, _ ...interface{})                {}
func (nop) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {}
func (nop) PanicRecovered(*http.Request, interface{})        {}
