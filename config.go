package httpserver

import (
	"context"
	"crypto/tls"
	"database/sql"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

type configuration struct {
	Context                  context.Context
	ContextShutdown          context.CancelFunc
	Handler                  http.Handler
	MaxRequestHeaderSize     int
	ReadRequestTimeout       time.Duration
	ReadRequestHeaderTimeout time.Duration
	WriteResponseTimeout     time.Duration
	IdleConnectionTimeout    time.Duration
	ShutdownTimeout          time.Duration
	ForceShutdownTimeout     time.Duration
	ListenNetwork            string
	ListenAddress            string
	ListenConfig             listenConfig
	ListenAdapter            func(net.Listener) net.Listener
	ListenReady              chan<- bool
	TLSConfig                *tls.Config
	HandlePanic              bool
	IgnoredErrors            []error
	Monitor                  monitor
	Logger                   logger
	ErrorLogger              logger
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
	return func(this *configuration) { this.ListenNetwork, this.ListenAddress = parseListenAddress(value) }
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
func (singleton) IgnoredErrors(value ...error) option {
	return func(this *configuration) { this.IgnoredErrors = value }
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
func (singleton) ForceShutdownTimeout(value time.Duration) option {
	return func(this *configuration) { this.ForceShutdownTimeout = value }
}
func (singleton) ListenConfig(value listenConfig) option {
	return func(this *configuration) { this.ListenConfig = value }
}
func (singleton) ListenAdapter(value func(net.Listener) net.Listener) option {
	return func(this *configuration) { this.ListenAdapter = value }
}
func (singleton) ListenReady(value chan<- bool) option {
	return func(this *configuration) { this.ListenReady = value }
}
func (singleton) Monitor(value monitor) option {
	return func(this *configuration) { this.Monitor = value }
}
func (singleton) Logger(value logger) option {
	return func(this *configuration) { this.Logger = value }
}
func (singleton) ErrorLogger(value logger) option {
	return func(this *configuration) { this.ErrorLogger = value }
}

// Deprecated: SocketConfig is deprecated.
func (singleton) SocketConfig(value listenConfig) option { return Options.ListenConfig(value) }

func (singleton) apply(options ...option) option {
	return func(this *configuration) {
		for _, item := range Options.defaults(options...) {
			item(this)
		}

		if this.HandlePanic {
			this.Handler = newRecoveryHandler(this.Handler, this.IgnoredErrors, this.Monitor, this.Logger)
		}

		this.Context, this.ContextShutdown = context.WithCancel(this.Context)
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
				ErrorLog:          newServerLogger(this.ErrorLogger),
			}
		}
	}
}
func (singleton) defaults(options ...option) []option {
	var defaultListenConfig = &net.ListenConfig{Control: func(_, _ string, conn syscall.RawConn) error {
		return conn.Control(func(descriptor uintptr) {
			_ = syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, socketReusePort, 1)
		})
	}}

	defaultNop := &nop{}

	return append([]option{
		Options.ListenAddress(":http"),
		Options.TLSConfig(nil),
		Options.MaxRequestHeaderSize(1024 * 2),
		Options.ReadRequestTimeout(time.Second * 5),
		Options.ReadRequestHeaderTimeout(time.Second),
		Options.WriteResponseTimeout(time.Second * 90),
		Options.IdleConnectionTimeout(time.Second * 30),
		Options.ShutdownTimeout(time.Second * 5),
		Options.ForceShutdownTimeout(time.Second),
		Options.HandlePanic(true),
		Options.IgnoredErrors(context.Canceled, context.DeadlineExceeded, sql.ErrTxDone),
		Options.Context(context.Background()),
		Options.Handler(defaultNop),
		Options.Monitor(defaultNop),
		Options.Logger(defaultNop),
		Options.ErrorLogger(defaultNop),
		Options.ListenConfig(defaultListenConfig),
		Options.ListenAdapter(nil),
		Options.ListenReady(nil),
	}, options...)
}

func parseListenAddress(value string) (string, string) {
	if parsed := parseURL(value); parsed == nil {
		return "tcp", value
	} else if strings.ToLower(parsed.Scheme) == "unix" {
		return "unix", value[len("unix://"):] // don't prepend slash which assumes full path because path might be relative
	} else {
		return coalesce(parsed.Scheme, "tcp"), coalesce(parsed.Host, parsed.Path)
	}
}
func parseURL(value string) *url.URL {
	value = strings.TrimSpace(value)
	if len(value) == 0 {
		return nil
	} else if parsed, err := url.Parse(value); err != nil {
		return nil
	} else {
		return parsed
	}
}
func coalesce(values ...string) string {
	for _, item := range values {
		if len(item) > 0 {
			return item
		}
	}
	return ""
}

type nop struct{}

func (*nop) Printf(_ string, _ ...any)                        {}
func (*nop) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {}
func (*nop) PanicRecovered(*http.Request, any)                {}
