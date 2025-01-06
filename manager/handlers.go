package manager

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	mgmtv1 "github.com/caldog20/calnet/proto/gen/management/v1"
)

func (s *Server) Login(
	ctx context.Context,
	req *connect.Request[mgmtv1.LoginRequest],
) (*connect.Response[mgmtv1.LoginResponse], error) {
	if !s.debugMode {
		err := validatePublicKey(req.Msg.GetPublicKey())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid public key"))
		}
		// TODO: validate machine id
	}

	peer, err := s.store.GetPeer()

	err := s.loginPeer(req.Msg)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	} else if errors.Is(err, ErrNotFound) {
		err = s.registerPeer(req.Msg)
		if err != nil {
			if errors.Is(err, ErrInvalidProvisionKey) {
				return nil, connect.NewError(connect.CodeUnauthenticated, err)
			} else {
				return nil, connect.NewError(connect.CodeInternal, err)
			}
		}
	}

	// TODO: Generate token for peer network update routine
	return connect.NewResponse(&mgmtv1.LoginResponse{}), nil
}

func (s *Server) NetworkUpdate(
	ctx context.Context,
	req *connect.Request[mgmtv1.NetworkUpdateRequest],
) (*connect.Response[mgmtv1.NetworkUpdateResponse], error) {
	// TODO: Validate token and peer public key
	publicKey := req.Msg.GetPublicKey()
	rev, err := s.GetPeerUpdateRevision(publicKey)
	t := time.NewTimer(time.Second * 5)
	select {
	case <-t.C:
		return connect.NewResponse(&mgmtv1.NetworkUpdateResponse{Revision: rev}), nil
	case update := <-s.GetPeerUpdate(publicKey):
		return connect.NewResponse(update), nil
	}

}
