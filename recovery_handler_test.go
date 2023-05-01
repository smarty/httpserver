package httpserver

import (
	"context"
	"database/sql"
	"errors"
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
	serveHTTPError    any

	panicRecoveredCount   int
	panicRecoveredRequest *http.Request
	panicRecoveredError   any

	logged []string
}

func (this *RecoveryHandlerFixture) Setup() {
	this.response = httptest.NewRecorder()
	this.request = httptest.NewRequest("GET", "/", nil)
	ignoredErrors := []error{context.Canceled, context.DeadlineExceeded, sql.ErrTxDone}
	this.handler = newRecoveryHandler(this, ignoredErrors, this, this)
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
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_ContextCancellation_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", context.Canceled)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_ContextDeadlineExceeded_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", context.DeadlineExceeded)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanic_SQL_TransactionDone_ReturnHTTP500() {
	this.serveHTTPError = fmt.Errorf("inner: %w", sql.ErrTxDone)

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 0)
	this.So(this.panicRecoveredRequest, should.BeNil)
	this.So(this.logged, should.BeEmpty)
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanicsWithError_ItShouldNotPanicButShouldNotifyMonitorAndReturnHTTP500() {
	this.serveHTTPError = errors.New("panic value")

	this.handler.ServeHTTP(this.response, this.request)

	this.So(this.response.Code, should.Equal, 500)
	this.So(this.panicRecoveredCount, should.Equal, 1)
	this.So(this.panicRecoveredRequest, should.Equal, this.request)
	if this.So(this.logged, should.HaveLength, 1) {
		this.So(this.logged[0], should.StartWith, "[ERROR] Recovered panic: panic value")
	}
}
func (this *RecoveryHandlerFixture) TestInnerHandlerPanicWithNonError_ItShouldNotPanicButShouldNotifyMonitorAndReturnHTTP500() {
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

func (this *RecoveryHandlerFixture) PanicRecovered(request *http.Request, err any) {
	this.panicRecoveredCount++
	this.panicRecoveredRequest = request
	this.panicRecoveredError = err
}

func (this *RecoveryHandlerFixture) Printf(format string, args ...any) {
	this.logged = append(this.logged, fmt.Sprintf(format, args...))
}
