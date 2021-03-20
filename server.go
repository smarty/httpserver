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
	softContext     context.Context
	softShutdown    context.CancelFunc
	shutdownTimeout time.Duration
	listenAddress   string
	listenConfig    listenConfig
	tlsConfig       *tls.Config
	httpServer      httpServer
	logger          logger
}

func newServer(config configuration) ListenCloser {
	softContext, softShutdown := context.WithCancel(config.Context)
	return defaultServer{
		config:          config,
		hardContext:     config.Context,
		softContext:     softContext,
		softShutdown:    softShutdown,
		shutdownTimeout: config.ShutdownTimeout,
		listenAddress:   config.ListenAddress,
		listenConfig:    config.SocketConfig,
		tlsConfig:       config.TLSConfig,
		httpServer:      config.HTTPServer,
		logger:          config.Logger,
	}
}

func (this defaultServer) Listen() {
	waiter := &sync.WaitGroup{}
	waiter.Add(2)
	defer waiter.Wait()

	go this.listen(waiter)
	go this.watchShutdown(waiter)
}
func (this defaultServer) listen(waiter *sync.WaitGroup) {
	defer waiter.Done()

	this.logger.Printf("[INFO] Listening for HTTP traffic on [%s]...", this.listenAddress)
	if listener, err := this.listenConfig.Listen(this.softContext, "tcp", this.listenAddress); err != nil {
		this.logger.Printf("[WARN] Unable to listen: [%s]", err)
	} else if err := this.httpServer.Serve(this.tryTLSListener(listener)); err == nil || err == http.ErrServerClosed {
		this.logger.Printf("[INFO] HTTP server concluded listening operations.")
	} else {
		this.logger.Printf("[WARN] Unable to listen: [%s]", err)
	}
}
func (this defaultServer) tryTLSListener(listener net.Listener) net.Listener {
	if this.tlsConfig == nil {
		return listener
	}

	return tls.NewListener(listener, this.tlsConfig)
}
func (this defaultServer) watchShutdown(waiter *sync.WaitGroup) {
	defer waiter.Done()

	<-this.softContext.Done()                                                  // waiting for soft context shutdown to occur
	ctx, cancel := context.WithTimeout(this.hardContext, this.shutdownTimeout) // wait until shutdownTimeout for shutdown
	defer cancel()
	this.logger.Printf("[INFO] Shutting down HTTP server...")
	_ = this.httpServer.Shutdown(ctx)
	this.logger.Printf("[INFO] HTTP server shutdown complete.")
}

func (this defaultServer) Close() error {
	this.softShutdown()
	return nil
}
