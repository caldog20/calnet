package stunservice

import (
	"context"
	"errors"
	"io"
	"log"
	"net"

	"github.com/pion/stun"
)

// TODO: Refactor this to hold some state and add some methods to match other implementations of controlserver

func ListenAndServe(ctx context.Context, listenAddr string) error {
	addr, err := net.ResolveUDPAddr("udp4", listenAddr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	log.Printf("stun server listening on %s", conn.LocalAddr().String())

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 1500)
	for {
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		if stun.IsMessage(buf[:n]) {
			msg := &stun.Message{}
			msg.Raw = buf[:n]
			err := msg.Decode()
			if err != nil {
				log.Println(err)
				continue
			}
			if msg.Type == stun.BindingRequest {
				xor := stun.XORMappedAddress{}
				xor.IP = raddr.IP
				xor.Port = raddr.Port
				err = xor.AddTo(msg)
				if err != nil {
					log.Println(err)
					continue
				}
				msg.Type = stun.BindingSuccess
				msg.Encode()
				_, err = conn.WriteToUDP(msg.Raw, raddr)
				if err != nil {
					return err
				}
			}
		}

	}
}
