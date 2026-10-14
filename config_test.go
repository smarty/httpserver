package httpserver

import (
	"net/http"
	"testing"

	"github.com/smarty/gunit"
	"github.com/smarty/gunit/assert/should"
)

func TestConfigurationFixture(t *testing.T) {
	gunit.Run(new(ConfigurationFixture), t)
}

type ConfigurationFixture struct {
	*gunit.Fixture
}

func (this *ConfigurationFixture) TestProtocolsAndHTTP2_DefaultToNil() {
	server := builtHTTPServer(New())

	this.So(server.Protocols, should.BeNil)
	this.So(server.HTTP2, should.BeNil)
}
func (this *ConfigurationFixture) TestProtocolsAndHTTP2_ArePassedToTheServerThisPackageBuilds() {
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	http2 := &http.HTTP2Config{MaxConcurrentStreams: 42}

	server := builtHTTPServer(New(
		Options.Protocols(protocols),
		Options.HTTP2(http2),
	))

	this.So(server.Protocols, should.Equal, protocols)
	this.So(server.HTTP2, should.Equal, http2)
}

func builtHTTPServer(value ListenCloser) *http.Server {
	return value.(*defaultServer).httpServer.(*http.Server)
}
