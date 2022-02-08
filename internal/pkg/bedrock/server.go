package bedrock

import (
	"net"
	"time"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/internal/app/infrared"
	"github.com/pires/go-proxyproto"
	"github.com/sandertv/go-raknet"
)

type Server struct {
	ID                 string
	Domains            []string
	Dialer             raknet.Dialer
	DialTimeout        time.Duration
	Address            string
	SendProxyProtocol  bool
	DialTimeoutMessage string
	WebhookIDs         []string
	Log                logr.Logger
}

type InfraredServer struct {
	Server
}

func (s InfraredServer) ID() string {
	return s.Server.ID
}

func (s InfraredServer) Domains() []string {
	return s.Server.Domains
}

func (s InfraredServer) WebhookIDs() []string {
	return s.Server.WebhookIDs
}

func (s *InfraredServer) SetLogger(log logr.Logger) {
	s.Server.Log = log
}

func (s InfraredServer) Dial() (*raknet.Conn, error) {
	c, err := s.Dialer.DialTimeout(s.Address, s.DialTimeout)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s InfraredServer) ProcessConn(c net.Conn) (infrared.ConnTunnel, error) {
	pc := c.(*ProcessedConn)
	rc, err := s.Dial()
	if err != nil {
		if err := s.handleDialTimeout(*pc); err != nil {
			return infrared.ConnTunnel{}, err
		}
		return infrared.ConnTunnel{}, err
	}

	if s.SendProxyProtocol {
		if err := writeProxyProtocolHeader(pc, rc); err != nil {
			return infrared.ConnTunnel{}, err
		}
	}

	if _, err := rc.Write(pc.readBytes); err != nil {
		rc.Close()
		return infrared.ConnTunnel{}, err
	}

	return infrared.ConnTunnel{
		Conn:       pc,
		RemoteConn: rc,
	}, nil
}

func (s InfraredServer) handleDialTimeout(c ProcessedConn) error {
	msg := infrared.ExecuteServerMessageTemplate(s.DialTimeoutMessage, c, &s)
	return c.disconnect(msg)
}

func writeProxyProtocolHeader(c, rc net.Conn) error {
	tp := proxyproto.UDPv4
	addr := c.RemoteAddr().(*net.UDPAddr)
	if addr.IP.To4() == nil {
		tp = proxyproto.UDPv6
	}

	header := &proxyproto.Header{
		Version:           2,
		Command:           proxyproto.PROXY,
		TransportProtocol: tp,
		SourceAddr:        c.RemoteAddr(),
		DestinationAddr:   rc.RemoteAddr(),
	}

	if _, err := header.WriteTo(rc); err != nil {
		return err
	}
	return nil
}
