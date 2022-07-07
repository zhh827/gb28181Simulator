package invite

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/lzh2nix/gb28181Simulator/internal/streams/packet"
	"github.com/nareix/joy4/format"
)

func init() {
	format.RegisterAll()
}

func TestSendRTP(t *testing.T) {
	//xlog := xlog.NewWith(context.Background())
	sendRTPPacket()
	t.Log("test complete")
}

func sendRTPPacket() {
	// "UDP"
	rtp := packet.NewRRtpTransfer("", packet.UDPTransfer, 0)
	// rtp := packet.NewRRtpTransfer("", packet.LocalCache, 0)

	// send ip,port and recv ip,port
	err := rtp.Service("192.168.124.5", "47.94.218.109", 5061, 1234)
	if err != nil {
		fmt.Println("connect failed, err = ", err)
	}
	f, err := os.Open("test.dat")
	if err != nil {
		fmt.Printf("read file error(%v)", err)
		rtp.Exit()
		return
	}

	defer func() {
		f.Close()
		rtp.Exit()
		rtp = nil
	}()

	buf, _ := ioutil.ReadAll(f)
	for {
		select {
		default:
			sendFile(buf, rtp)
		}

	}
}

func sendFile(buf []byte, rtp *packet.RtpTransfer) {
	last := 0
	var pts uint64 = 0
	for i := 4; i < len(buf); i++ {
		if isPsHead(buf[i : i+4]) {
			rtp.SendPSdata(buf[last:i], false, pts)
			pts += 40
			time.Sleep(time.Millisecond * 40)
			last = i
		}
	}
}
