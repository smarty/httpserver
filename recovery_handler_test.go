package httpserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestRecoveryHandlerFixture(t *testing.T) {
	gunit.Run(new(RecoveryHandlerFixture), t)
}

type RecoveryHandlerFixture struct {
	*gunit.Fixture

	handler http.Handler

	response *httptest.ResponseRecorder
	request  *http.Request

	serveHTTPCount    int
	serveHTTPResponse http.ResponseWriter
	serveHTTPRequest  *http.Request
	serveHTTPError    interface{}

	panicRecoveredCount   int
	panicRecoveredRequest *http.Request
	panicRecoveredError   interface{}

	logged []string
}

func (this *RecoveryHandlerFixture) Setup() {
	this.response = httptest.NewRecorder()
	this.request = httptest.NewRequest("GET", "/", nil)
	this.handler = newRecoveryHandler(this, this, this)
}

func (this *RecoveryHandlerFixture) TestInnerHandlerCalled() {
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.serveHTTPCount, should.Equal, 1)
	this.So(this.serveHTTPResponse, should.Equal, this.response)
	this.So(this.serveHTTPRequest, should.Equal, this.request)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerDoesNotPanic_NotRecoveryNecessary() {
	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 200)
	this.So(this.panicRecoveredCount, should.Equal, 0)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_ItShouldNotPanicButShouldNotifyMonitorAndReturnHTTP500() {
	this.serveHTTPError = "panic value"

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func (this *RecoveryHandlerFixture) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	this.serveHTTPCount++
	this.serveHTTPResponse = response
	this.serveHTTPRequest = request
	if this.serveHTTPError != nil {
		panic(this.serveHTTPError)
	}
}

func (this *RecoveryHandlerFixture) PanicRecovered(request *http.Request, err interface{}) {
	this.panicRecoveredCount++
	this.panicRecoveredRequest = request
	this.panicRecoveredError = err
}

func (this *RecoveryHandlerFixture) Printf(format string, args ...interface{}) {
	this.logged = append(this.logged, fmt.Sprintf(format, args...))
}
