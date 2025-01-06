package manager

import (
	"context"
	"log"

	"connectrpc.com/connect"
)

func NewLogInterceptor() connect.UnaryInterceptorFunc {
	interceptor := func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(
			ctx context.Context,
			req connect.AnyRequest,
		) (connect.AnyResponse, error) {
			log.Printf("connect %s request from peer: %s", req.HTTPMethod(), req.Peer().Addr)
			log.Println("request headers: ", req.Header())
			log.Println("request method: ", req.Spec().Procedure)
			log.Println("request body: ", req.Any())
			return next(ctx, req)
		})
	}

	return connect.UnaryInterceptorFunc(interceptor)
}
