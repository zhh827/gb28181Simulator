package invite

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jart/gosip/sdp"
	"github.com/jart/gosip/sip"
	"github.com/jart/gosip/util"
	"github.com/lzh2nix/gb28181Simulator/internal/config"
	"github.com/lzh2nix/gb28181Simulator/internal/streams/packet"
	"github.com/lzh2nix/gb28181Simulator/internal/transport"
	"github.com/lzh2nix/gb28181Simulator/internal/version"
	"github.com/qiniu/x/xlog"

	"github.com/nareix/joy4/format"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/av/avutil"
)

const (
	idle = iota
	//	proceeding // recv invite, send 100 trying
	completed // send 200 OK
	confirmed // recv ACK
)

type Leg struct {
	callID  string
	fromTag string
	toTag   string
}
type sdpRemoteInfo struct {
	ssrc  int
	ip    string
	port  int
	proto string
	lPort int
}
type Invite struct {
	cfg    *config.Config
	state  int32
	leg    *Leg
	remote *sdpRemoteInfo
}

func init() {
	format.RegisterAll()
}
func NewInvite(cfg *config.Config) *Invite {
	rand.Seed(time.Now().UnixNano())
	return &Invite{cfg: cfg, state: idle}
}

func (inv *Invite) HandleMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	if m.CSeqMethod == sip.MethodInvite && strings.ToUpper(m.Payload.ContentType()) == "APPLICATION/SDP" {
		inv.InviteMsg(xlog, tr, m)
		return
	}
	if m.CSeqMethod == sip.MethodAck {
		inv.AckMsg(xlog, tr, m)
		return

		// send rtp msg
	}
	if m.CSeqMethod == sip.MethodBye {
		inv.ByeMsg(xlog, tr, m)
		return
	}

	xlog.Info("recv msg at ", inv.state, m)
}
func (inv *Invite) InviteMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	// only handle invite idle state
	xlog.Info("Sever------>Invite--->Client")
	if atomic.LoadInt32(&inv.state) != idle {
		return
	}
	sdp, err := sdp.Parse(string(m.Payload.Data()))
	if err != nil {
		xlog.Error("parse sdp failed, err = ", err)
	}
	fmt.Println(sdp.Attrs, sdp.Addr, sdp.Video.Port, sdp.Video.Proto, sdp.Session)
	r := &sdpRemoteInfo{
		ssrc:  ssrc(sdp),
		ip:    sdp.Addr,
		port:  int(sdp.Video.Port),
		lPort: randomFromStartEnd(10000, 65535),
	}
	if strings.HasPrefix(sdp.Video.Proto, "TCP") {
		r.proto = "TCP"
	} else {
		r.proto = "UDP"
	}

	inv.remote = r
	laHost := tr.Conn.LocalAddr().(*net.UDPAddr).IP.String()
	laPort := tr.Conn.LocalAddr().(*net.UDPAddr).Port
	resp := inv.makeRespFromReq(laHost, laPort, m, true, 200)
	inv.leg = &Leg{m.CallID, m.From.Param.Get("tag").Value, resp.To.Param.Get("tag").Value}
	atomic.StoreInt32(&inv.state, completed)
	xlog.Info("Client------>200OK(Invite)--->Server")
	tr.Send <- resp
}
func (inv *Invite) makeRespFromReq(localHost string, localPort int, req *sip.Msg, invite bool, code int) *sip.Msg {
	resp := &sip.Msg{
		Status:     code,
		From:       req.From.Copy(),
		To:         req.To.Copy(),
		CallID:     req.CallID,
		CSeq:       req.CSeq,
		CSeqMethod: req.CSeqMethod,
		UserAgent:  version.Version(),
		Via: &sip.Via{
			Version:  "2.0",
			Protocol: "SIP",
			Host:     localHost,
			Port:     uint16(localPort),
			Param:    &sip.Param{Name: "branch", Value: req.Via.Param.Get("branch").Value},
		},
	}

	if invite && code == 200 {
		resp.To.Tag()
		sdp := &sdp.SDP{
			Origin:  sdp.Origin{User: inv.cfg.GBID, Addr: localHost},
			Session: "play",
			Addr:    localHost,
			Video: &sdp.Media{
				//Proto:  inv.remote.proto + "/RTP/AVP",
				Proto: "TCP/RTP/AVP",

				Codecs: []sdp.Codec{{PT: uint8(96), Rate: 90000, Name: "PS"}},
				Port:   uint16(inv.remote.lPort)},
			SendOnly: true,
			Other:    [][2]string{[2]string{"y", strconv.Itoa(inv.remote.ssrc)}},
		}
		resp.Payload = sdp
	} else {
		toTag := util.GenerateTag()
		if inv.leg != nil {
			toTag = inv.leg.toTag
		}
		resp.To.Param = &sip.Param{Name: "tag", Value: toTag}
	}
	return resp
}
func ssrc(sdp *sdp.SDP) int {
	for _, i := range sdp.Other {
		if i[0] == "y" {
			ssrc, _ := strconv.ParseInt(i[1], 10, 64)
			return int(ssrc)
		}
	}
	return 0
}

func (inv *Invite) AckMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	// only handle invite idle state
	if atomic.LoadInt32(&inv.state) != completed ||
		inv.leg.callID != m.CallID ||
		!strings.EqualFold(inv.leg.fromTag, m.From.Param.Get("tag").Value) {
		return
	}
	xlog.Info("Server------>ACK--->Client")
	atomic.StoreInt32(&inv.state, confirmed)
	// start send rtp
	go inv.sendRTPPacket(xlog)
}

func randomFromStartEnd(min, max int) int {

	return rand.Intn(max-min+1) + min
}
func (inv *Invite) sendRTPPacket(xlog *xlog.Logger) {
	var rtp *packet.RtpTransfer
	if inv.remote.proto == "UDP" {
		rtp = packet.NewRRtpTransfer("", packet.UDPTransfer, inv.remote.ssrc)
	} else {
		//rtp = packet.NewRRtpTransfer("", packet.UDPTransfer, inv.remote.ssrc)
		rtp = packet.NewRRtpTransfer("", packet.TCPTransferActive, inv.remote.ssrc)
	}
	// send ip,port and recv ip,port
	err := rtp.Service("100.100.34.52", inv.remote.ip, inv.remote.lPort, inv.remote.port)
	if err != nil {
		xlog.Info("connect failed, err = ", err)
	}
	//rtp.Service("100.100.57.239", "101.132.180.234", inv.remote.lPort, 10000)

	f, err := avutil.Open("Big_Buck_Bunny_1080_10s_1MB.mp4")
	if err != nil {
		xlog.Errorf("read file error(%v)", err)
		rtp.Exit()
		return
	}

	var pts uint64 = 10000
	streams, _ := f.Streams()
	var vindex int8
	for i, stream := range streams {
		if stream.Type() == av.H264 {
			vindex = int8(i)
			break
		}
	}
	defer func() {
		f.Close()
		rtp.Exit()
	}()
	for {
		var pkt av.Packet
		var err error
		if pkt, err = f.ReadPacket(); err != nil {
			xlog.Errorf("read packet error(%v)", err)
			break
		}
		if pkt.Idx != vindex {
			continue
		}
		rtp.Send2data(pkt.Data, pkt.IsKeyFrame, pts)
		pts += 40
		fmt.Println("fdfafdafas")
		time.Sleep(time.Millisecond * 40)
	}

	return
}
func (inv *Invite) ByeMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	// only handle invite idle state
	if m.IsResponse() {
		return
	}
	xlog.Info("Server------>Bye--->Client")
	laHost := tr.Conn.LocalAddr().(*net.UDPAddr).IP.String()
	laPort := tr.Conn.LocalAddr().(*net.UDPAddr).Port
	if atomic.LoadInt32(&inv.state) != confirmed ||
		inv.leg.callID != m.CallID ||
		!strings.EqualFold(inv.leg.fromTag, m.From.Param.Get("tag").Value) {
		resp := inv.makeRespFromReq(laHost, laPort, m, false, 481)
		xlog.Info("Client------>481(Bye)--->Server")
		tr.Send <- resp
		atomic.StoreInt32(&inv.state, idle)
		return
	}
	resp := inv.makeRespFromReq(laHost, laPort, m, false, 200)
	atomic.StoreInt32(&inv.state, idle)
	xlog.Info("Client------>200OK(Bye)--->Server")
	tr.Send <- resp
}
