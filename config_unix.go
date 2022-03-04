package httpserver

// socketReusePort indicates that a given port is available to be bound by multiple discrete processes at the same time.
// While only one socket is active at any given time and the first socket to be bound must release in order to allow for
// traffic to proceed to the second socket, the bind operation will not fail.
const socketReusePort = 15

// NOTE: Unlike TCP sockets, UNIX Domain Sockets (UDS) do not have any concept of "reuse port". This means that once a
// listener has bound to a socket at a given path, no other processes can bind to that same socket. POSIX has a
// provision which allows a process to fork such that a child can inherit the socket but this isn't trivial in a Go app.
// Therefore, one potential workaround is a blue/green style of sockets where there's a blue socket and a green socket
// and a given listener will attempt to bind to either of those (ideally blue then green) and then the client side will
// try to bind to whichever is available. This could easily be indicated using the url.URL semantics using a query param
// e.g. unix:///tmp/app.sock?style=bluegreen (or similar) (not the triple slash, e.g. 2 slashes for scheme and the
// third slash to indicate an absolute path). Or we could use unix:///tmp/app-[bluegreen].sock where the bluegreen is
// replaced dynamically by the caller once a connection is made.

// One challenge with the above is when we have transient clients that don't track much state.
