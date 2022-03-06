package infrared

import (
	"net"

	"github.com/go-logr/logr"
	"github.com/haveachin/infrared/pkg/event"
)

type ProxyConfig interface {
	LoadGateways() ([]Gateway, error)
	LoadServers() ([]Server, error)
	LoadConnProcessor() (ConnProcessor, error)
	LoadProxySettings() (ProxySettings, error)
}

type ProxyChannelCaps struct {
	ConnProcessor int
	Server        int
	ConnPool      int
}

type ProxySettings struct {
	ChannelCaps ProxyChannelCaps
	CPNCount    int
}

type Proxy struct {
	settings      ProxySettings
	gateways      []Gateway
	cpnPool       CPNPool
	serverGateway ServerGateway
	connPool      ConnPool
	cpnCh         chan net.Conn
	srvCh         chan ProcessedConn
	poolCh        chan ConnTunnel
}

func NewProxy(cfg ProxyConfig) (Proxy, error) {
	gws, err := cfg.LoadGateways()
	if err != nil {
		return Proxy{}, err
	}

	cp, err := cfg.LoadConnProcessor()
	if err != nil {
		return Proxy{}, err
	}

	srvs, err := cfg.LoadServers()
	if err != nil {
		return Proxy{}, err
	}

	stg, err := cfg.LoadProxySettings()
	if err != nil {
		return Proxy{}, err
	}

	chCaps := stg.ChannelCaps
	cpnCh := make(chan net.Conn, chCaps.ConnProcessor)
	srvCh := make(chan ProcessedConn, chCaps.Server)
	poolCh := make(chan ConnTunnel, chCaps.ConnPool)

	return Proxy{
		settings: stg,
		gateways: gws,
		cpnPool: CPNPool{
			CPN: CPN{
				ConnProcessor: cp,
				In:            cpnCh,
				Out:           srvCh,
			},
		},
		serverGateway: ServerGateway{
			Gateways: gws,
			Servers:  srvs,
		},
		connPool: ConnPool{},
		cpnCh:    cpnCh,
		srvCh:    srvCh,
		poolCh:   poolCh,
	}, nil
}

func (p *Proxy) ListenAndServe(log logr.Logger) {
	p.cpnPool.CPN.Log = log
	p.cpnPool.CPN.EventBus = event.DefaultBus
	p.cpnPool.SetSize(p.settings.CPNCount)

	for _, gw := range p.gateways {
		gw.SetLogger(log)
		go ListenAndServe(gw, p.cpnCh)
	}

	p.connPool.Log = log
	go p.connPool.Start(p.poolCh)

	for _, srv := range p.serverGateway.Servers {
		srv.SetLogger(log)
	}

	p.serverGateway.Log = log
	p.serverGateway.Start(p.srvCh, p.poolCh)
}
