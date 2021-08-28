package httpserver

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

type defaultServer struct {
	config          configuration
	hardContext     context.Context
	hardShutdown    context.CancelFunc
	softContext     context.Context
	softShutdown    context.CancelFunc
	shutdownTimeout time.Duration
	forcedTimeout   time.Duration
	listenAddress   string
	listenConfig    listenConfig
	listenAdapter   func(net.Listener) net.Listener
	tlsConfig       *tls.Config
	httpServer      httpServer
	logger          logger
}

func newServer(config configuration) ListenCloser {
	softContext, softShutdown := context.WithCancel(config.Context)
	return &defaultServer{
		config:          config,
		hardContext:     config.Context,
		hardShutdown:    config.ContextShutdown,
		softContext:     softContext,
		softShutdown:    softShutdown,
		shutdownTimeout: config.ShutdownTimeout,
		forcedTimeout:   config.ForceShutdownTimeout,
		listenAddress:   config.ListenAddress,
		listenConfig:    config.ListenConfig,
		listenAdapter:   config.ListenAdapter,
		tlsConfig:       config.TLSConfig,
		httpServer:      config.HTTPServer,
		logger:          config.Logger,
	}
}

func (this *defaultServer) Listen() {
	waiter := &sync.WaitGroup{}
	waiter.Add(2)
	defer waiter.Wait()

	go this.listen(waiter)
	go this.watchShutdown(waiter)
}
func (this *defaultServer) listen(waiter *sync.WaitGroup) {
	defer waiter.Done()

	if len(this.listenAddress) == 0 {
		return
	}

	this.logger.Printf("[INFO] Listening for HTTP traffic on [%s]...", this.listenAddress)
	if listener, err := this.newListener(); err != nil {
		this.logger.Printf("[WARN] Unable to listen: [%s]", err)
	} else if err = this.httpServer.Serve(listener); err == nil || err == http.ErrServerClosed {
		this.logger.Printf("[INFO] HTTP server concluded listening operations.")
	} else {
		this.logger.Printf("[WARN] Unable to listen: [%s]", err)
	}
}
func (this *defaultServer) newListener() (net.Listener, error) {
	listener, err := this.listenConfig.Listen(this.softContext, "tcp", this.listenAddress)
	if err != nil {
		return nil, err
	}

	if this.listenAdapter != nil {
		listener = this.listenAdapter(listener)
	}

	if this.tlsConfig != nil {
		listener = tls.NewListener(listener, this.tlsConfig)
	}

	return listener, nil
}
func (this *defaultServer) watchShutdown(waiter *sync.WaitGroup) {
	var shutdownError error
	defer func() {
		defer waiter.Done()
		this.hardShutdown()
		this.awaitOutstandingRequests(shutdownError)
	}()

	<-this.softContext.Done()                                                  // waiting for soft context shutdown to occur
	ctx, cancel := context.WithTimeout(this.hardContext, this.shutdownTimeout) // wait until shutdownTimeout for shutdown
	defer cancel()
	this.logger.Printf("[INFO] Shutting down HTTP server...")
	shutdownError = this.httpServer.Shutdown(ctx)
}
func (this *defaultServer) awaitOutstandingRequests(err error) {
	defer this.logger.Printf("[INFO] HTTP server shutdown complete.")

	if err == nil {
		return
	}

	// 1+ outstanding request(s) is/are still being processed, if the request.Context() cancellation is considered by
	// the http.Handler, let's give a moment longer to complete the run through the configured http.Handler pipeline.
	this.logger.Printf("[INFO] HTTP request(s) in flight after server shutdown, waiting for %s...", this.forcedTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), this.forcedTimeout)
	defer cancel()
	<-ctx.Done()
}

func (this *defaultServer) Close() error {
	this.softShutdown()
	return nil
}
