package invite

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"my28181/config"

	"my28181/media/streams/packet"
	"my28181/transport"

	"my28181/message/version"

	"github.com/jart/gosip/sdp"
	"github.com/jart/gosip/sip"
	"github.com/jart/gosip/util"
	"github.com/qiniu/x/log"
	"github.com/qiniu/x/xlog"

	"github.com/nareix/joy4/format"
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
	lip   string
}

type Invite struct {
	cfg    *config.Config
	state  int32
	leg    *Leg
	remote *sdpRemoteInfo
	rtp    *packet.RtpTransfer
	byed   chan bool
}

func init() {
	format.RegisterAll()
}

func NewInvite(cfg *config.Config) *Invite {
	rand.Seed(time.Now().UnixNano())
	return &Invite{cfg: cfg, state: idle, byed: make(chan bool)}
}

func (inv *Invite) HandleMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	xlog.Infof("收到消息,state: %d 内容: \n %s\n ", inv.state, m)
	// 处理Invite消息,消息类型是 "APPLICATION/SDP"
	if m.CSeqMethod == sip.MethodInvite && strings.ToUpper(m.Payload.ContentType()) == "APPLICATION/SDP" {
		inv.InviteMsg(xlog, tr, m)
		return
	}

	// 处理ACK消息,收到server的 INVITE 后的ACK准备进行推流
	if m.CSeqMethod == sip.MethodAck {
		inv.AckMsg(xlog, tr, m)
		return
		// send rtp msg
	}

	// 处理Bye消息
	if m.CSeqMethod == sip.MethodBye {
		inv.RecvByeMsg(xlog, tr, m)
		return
	}

}

func (inv *Invite) InviteMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	if atomic.LoadInt32(&inv.state) != idle {
		return
	}
	sdp, err := sdp.Parse(string(m.Payload.Data()))
	if err != nil {
		xlog.Error("parse sdp failed, err = ", err)
	}
	laHost := tr.Conn.LocalAddr().(*net.UDPAddr).IP.String()
	laPort := tr.Conn.LocalAddr().(*net.UDPAddr).Port
	r := &sdpRemoteInfo{
		ssrc:  ssrc(sdp),
		ip:    sdp.Addr,
		port:  int(sdp.Video.Port),
		lPort: randomFromStartEnd(10000, 65535),
		lip:   laHost,
	}
	if strings.HasPrefix(sdp.Video.Proto, "TCP") {
		r.proto = "TCP"
	} else {
		r.proto = "UDP"
	}

	inv.remote = r
	resp := inv.makeRespFromReq(laHost, laPort, m, true, 200)
	// 缺少Contact字段,视频聚合平台不能正常响应ACK消息
	resp.Contact = m.To
	inv.leg = &Leg{m.CallID, m.From.Param.Get("tag").Value, resp.To.Param.Get("tag").Value}
	atomic.StoreInt32(&inv.state, completed)
	xlog.Info("发送INVITE RESPONE消息 \n", resp)
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
			Other:    [][2]string{{"y", strconv.Itoa(inv.remote.ssrc)}},
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
	// 只处理 invite idle 状态的ack消息
	if atomic.LoadInt32(&inv.state) != completed ||
		inv.leg.callID != m.CallID || !strings.EqualFold(inv.leg.fromTag, m.From.Param.Get("tag").Value) {
		return
	}
	xlog.Debug("收到Server的INVITE ACK,准备进行推流 \n", m)
	atomic.StoreInt32(&inv.state, confirmed)
	// 发送rtp数据包
	go inv.sendRTPPacket(xlog, tr, m)
}

func randomFromStartEnd(min, max int) int {
	return rand.Intn(max-min+1) + min
}

func (inv *Invite) sendRTPPacket(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	inv.rtp = nil
	xlog.Infof("准备开始创建RRtpTransfer对象: %s\n", inv.rtp.GetVarInfo())
	if inv.remote.proto == "UDP" {
		inv.rtp = packet.NewRRtpTransfer("", packet.UDPTransfer, inv.remote.ssrc)
		xlog.Infof("创建UDP inv.rtp对象成功: %v\n", &inv.rtp)
	} else {
		inv.rtp = packet.NewRRtpTransfer("", packet.TCPTransferActive, inv.remote.ssrc)
		xlog.Infof("创建TCP inv.rtp对象成功: %v\n", &inv.rtp)
	}

	// 根据目标ip端口 创建go携程 channel接收数据 socket发送数据
	err := inv.rtp.Service(inv.remote.lip, inv.remote.ip, inv.remote.lPort, inv.remote.port)
	if err != nil {
		xlog.Info("连接失败: ", err)
	}
	// 打开视频文件
	f, err := os.Open("test.dat")
	if err != nil {
		xlog.Errorf("打开文件错误: %s ,%s", "test.dat", err)
		inv.rtp.Exit()
		return
	}

	// 播放完成后的清理
	defer func() {
		err := recover()
		if err != nil {
			fmt.Println("播放错误: ", err)
		}
		xlog.Info("发送rtp数据完成,清除 inv.rtp")
		// 关闭文件 网络socket
		f.Close()
		inv.rtp.Exit()
		inv.state = idle
	}()

	buf, err := ioutil.ReadAll(f)
	if err != nil {
		xlog.Errorf("读取视频文件错误: %s ,%s", "test.dat", err)
		return
	} else {
		xlog.Infof("读取视频文件成功, 视频大小: %d", len(buf))
	}

	// 等待 bye消息,终止数据发送
	go func() {
		for {
			<-inv.byed
			break
		}
	}()
	inv.sendFile(buf)
	// 播放完成,发送Bye消息
	inv.SendByeMsg(xlog, tr, m)
	xlog.Info("播放完成 \n")
}

func (inv *Invite) sendFile(buf []byte) {
	last := 0
	var pts uint64 = 0
	log.Printf("准备开始推送视频流,当前inv.byed: %d, inv.state: %d", inv.byed, inv.state)
	for i := 4; i < len(buf); i++ {
		if inv.state == idle {
			return
		}
		if isPsHead(buf[i : i+4]) {
			inv.rtp.SendPSdata(buf[last:i], false, pts)
			pts += 40
			time.Sleep(time.Millisecond * 40)
			last = i
		}
	}
}

func isPsHead(buf []byte) bool {
	h := []byte{0, 0, 1, 186}
	if len(buf) == 4 {
		for i := 0; i < 4; i++ {
			if buf[i] != h[i] {
				return false
			}
		}
		return true
	}
	return false
}

func (inv *Invite) RecvByeMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	if m.IsResponse() {
		xlog.Debug("接收到主动发送的Bye消息响应消息:", m)
		// 处理主动发送bye消息，对端响应后发动ack?
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
	inv.byed <- true
}

func (inv *Invite) SendByeMsg(xlog *xlog.Logger, tr *transport.Transport, m *sip.Msg) {
	// 播放完成时主动发送 Bye消息,使用invite消息生成
	xlog.Debug("发送Bye消息, invite消息内容: ", m)

	laHost := tr.Conn.LocalAddr().(*net.UDPAddr).IP.String()
	laPort := tr.Conn.LocalAddr().(*net.UDPAddr).Port
	req := &sip.Msg{
		CSeq:       m.CSeq + 1,
		CallID:     m.CallID,
		Method:     sip.MethodBye,
		CSeqMethod: sip.MethodBye,
		Request:    m.Request,
		Via: &sip.Via{
			Version:  "2.0",
			Protocol: "SIP",
			Host:     laHost,
			Port:     uint16(laPort),
			Param:    &sip.Param{Name: "branch", Value: m.Via.Param.Value},
		},
		To: m.From,
		From: &sip.Addr{
			Uri: &sip.URI{
				User: m.To.Uri.User,
				Host: m.To.Uri.Host,
			},
		},
	}
	req.From.Param = &sip.Param{Name: "tag", Value: inv.leg.toTag}
	xlog.Debug("发送Bye消息内容: ", req)
	tr.Send <- req
	inv.byed <- true
}
