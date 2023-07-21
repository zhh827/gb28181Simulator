package message

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"my28181/config"
	"my28181/message/catalog"
	"my28181/message/invite"
	"my28181/message/reg"
	"my28181/transport"

	"github.com/jart/gosip/sip"
	"github.com/qiniu/x/xlog"
)

const (
	Unknow        = "Unknow"
	CataLog       = "Catalog"
	DeviceControl = "DeviceControl"
)

var msgTypeRegexp = regexp.MustCompile(`<CmdType>([\w]+)</CmdType>`)

type Service struct {
	tr   *transport.Transport
	xlog *xlog.Logger

	regSrv     *reg.Registar
	catalogSrv *catalog.Catalog
	inviteSrv  *invite.Invite
}

func NewService(xlog *xlog.Logger, cfg *config.Config) (*Service, error) {
	tr, err := transport.StartSip(xlog, cfg.ServerAddr, cfg.Transport)
	if err != nil {
		return nil, err
	}
	reg, _ := reg.NewRegistar(cfg)
	catalog := catalog.NewCatalog(cfg)
	invite := invite.NewInvite(cfg)
	go reg.Run(xlog, tr)
	srv := &Service{
		tr:         tr,
		xlog:       xlog,
		regSrv:     reg,
		catalogSrv: catalog,
		inviteSrv:  invite,
	}
	return srv, nil
}

func msgType(m *sip.Msg) string {
	if len(m.Payload.Data()) != 0 && strings.ToUpper(m.Payload.ContentType()) == "APPLICATION/MANSCDP+XML" {
		cmdType := msgTypeRegexp.FindString(string(m.Payload.Data()))
		cmdType = strings.TrimPrefix(cmdType, "<CmdType>")
		return strings.TrimSuffix(cmdType, "</CmdType>")
	}
	return Unknow
}

func (s *Service) HandleIncommingMsg() {
	s.hookSignals()
	for m := range s.tr.Recv {
		// 如果是响应消息,构建respone回复
		if m.IsResponse() && s.regSrv.HandleResponse(s.xlog, s.tr, m) {
			continue
		}
		// 如果不是响应消息, 消息类型为message的
		if !m.IsResponse() && m.CSeqMethod == sip.MethodMessage {
			switch msgType(m) {
			case CataLog:
				s.catalogSrv.Handle(s.xlog, s.tr, m)
			case Unknow:
				fmt.Println("未知消息: ", m)
			}
		}
		// 处理 Invite Bye Ack消息
		if m.CSeqMethod == sip.MethodInvite || m.CSeqMethod == sip.MethodBye || m.CSeqMethod == sip.MethodAck {
			s.inviteSrv.HandleMsg(s.xlog, s.tr, m)
		}
	}
}

func (s *Service) Close() {
	s.regSrv.CloseChan <- true
	time.Sleep(time.Millisecond * 20)
}

// OnSignal will be called when a OS-level signal is received.
func (s *Service) onSignal(sig os.Signal) {
	switch sig {
	case syscall.SIGTERM:
		fallthrough
	case syscall.SIGINT:
		s.xlog.Infof("received signal %s, exiting...", sig.String())
		s.Close()
		os.Exit(0)
	}
}

// OnSignal starts the signal processing and makes su
func (s *Service) hookSignals() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range c {
			s.onSignal(sig)
		}
	}()
}
