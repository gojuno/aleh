package httpclient

import (
	"context"
	"net"
	"net/http"
)

func SocketClient(socketPath string) http.Client {
	return http.Client{
		Transport: &http.Transport{

			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}
}
