package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestServerFixture(t *testing.T) {
	gunit.Run(new(ServerFixture), t)
}

type ServerFixture struct {
	*gunit.Fixture

	masterContext   context.Context
	shutdownTimeout time.Duration
	server          ListenCloser

	listenCount   int
	listenContext context.Context
	listenNetwork string
	listenAddress string
	listenError   error

	serveCount    int
	serveContext  context.Context
	serveListener net.Listener
	serveError    error

	shutdownCount   int
	shutdownContext context.Context
	shutdownError   error

	mutex  sync.Mutex
	logged []string
}

func (this *ServerFixture) Setup() {
	this.masterContext = context.Background()
	this.initialize()
}
func (this *ServerFixture) initialize() {
	this.server = New(
		Options.Context(this.masterContext),
		Options.SocketConfig(this),
		Options.HTTPServer(this),
		Options.ShutdownTimeout(this.shutdownTimeout),
		Options.ListenAddress("my-listen-address"),
		Options.Logger(this),
	)
}

func (this *ServerFixture) TestWhenListenerFails_ItShouldNotServe() {
	this.listenError = errors.New("")
	this.masterContext = context.WithValue(context.Background(), "key", "master-context")
	this.initialize()

	go func() {
		time.Sleep(time.Millisecond)
		_ = this.server.Close()
	}()

	this.server.Listen()

	this.So(this.serveCount, should.Equal, 0)
	this.So(this.listenCount, should.Equal, 1)
	this.So(this.listenContext, should.NotEqual, this.masterContext) // inherits from master context
	this.So(this.listenContext.Value("key"), should.Equal, "master-context")
	this.So(this.listenNetwork, should.Equal, "tcp")
	this.So(this.listenAddress, should.Equal, "my-listen-address")
}
func (this *ServerFixture) TestWhenServerFails_ItShouldWaitUntilCloseIsExplicitlyInvoked() {
	this.serveError = errors.New("")
	this.initialize()

	go func() {
		time.Sleep(time.Millisecond * 5)
		_ = this.server.Close()
	}()
	started := time.Now().UTC()
	this.server.Listen()

	this.So(time.Since(started), should.BeGreaterThan, time.Millisecond*5)
}

func (this *ServerFixture) TestWhenMasterContextConcludes_CloseNeedNotBeExplicitlyInvoked() {
	this.masterContext = context.WithValue(context.Background(), "key", "master-context")
	ctx, shutdown := context.WithCancel(this.masterContext)
	this.masterContext = ctx
	this.initialize()

	shutdown()
	this.server.Listen()

	this.So(this.shutdownCount, should.Equal, 1)
	this.So(this.shutdownContext, should.NotEqual, this.masterContext)
	this.So(this.shutdownContext.Value("key"), should.Equal, "master-context")
}
func (this *ServerFixture) TestWhenServerClosing_ItAllowsServerTimeToConcludeListenOperations() {
	this.shutdownTimeout = time.Millisecond * 5
	this.initialize()

	go func() { _ = this.server.Close() }()
	started := time.Now().UTC()
	this.server.Listen()

	this.So(time.Since(started), should.BeGreaterThan, this.shutdownTimeout)
}

func (this *ServerFixture) TestWhenServeFails_ItShouldLogWarning() {
	const failureMessage = "this message should be logged"
	this.serveError = errors.New(failureMessage)

	go func() { _ = this.server.Close() }()
	this.server.Listen()

	this.So(this.logContainsMessage(failureMessage), should.BeTrue)
}
func (this *ServerFixture) TestServerShutdownCompletes_ItShouldNotLogWarning() {
	this.serveError = http.ErrServerClosed

	go func() { _ = this.server.Close() }()
	this.server.Listen()

	this.So(this.logContainsMessage("[WARN]]"), should.BeFalse)
}
func (this *ServerFixture) logContainsMessage(text string) bool {
	for _, message := range this.logged {
		if strings.Contains(message, text) {
			return true
		}
	}

	return false
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (this *ServerFixture) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	this.listenCount++
	this.listenContext = ctx
	this.listenNetwork = network
	this.listenAddress = address
	return this, this.listenError
}
func (this *ServerFixture) Serve(listener net.Listener) error {
	this.serveCount++
	this.serveListener = listener
	return this.serveError
}
func (this *ServerFixture) Shutdown(ctx context.Context) error {
	this.shutdownCount++
	this.shutdownContext = ctx
	<-ctx.Done()
	return this.shutdownError
}

func (this *ServerFixture) Printf(format string, args ...interface{}) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	this.logged = append(this.logged, fmt.Sprintf(format, args...))
}

func (this *ServerFixture) Accept() (net.Conn, error) { panic("nop") }
func (this *ServerFixture) Close() error              { panic("nop") }
func (this *ServerFixture) Addr() net.Addr            { panic("nop") }
