package main

import (
	"context"
	"fmt"
	"os"

	cli "github.com/jawher/mow.cli"
	"github.com/lzh2nix/gb28181Simulator/internal/config"
	"github.com/lzh2nix/gb28181Simulator/internal/useragent"
	"github.com/lzh2nix/gb28181Simulator/internal/version"
	"github.com/qiniu/x/xlog"

	"github.com/lzh2nix/gb28181Simulator/internal/streams/packet"

	"github.com/nareix/joy4/av"
	"github.com/nareix/joy4/format"

	"github.com/nareix/joy4/av/avutil"
)

func init() {
	format.RegisterAll()
}

func main() {
	// 转化视频文件为 ps流数据
	// mp4ConvertPsStream()

	xlog.SetOutputLevel(0)
	xlog.SetFlags(xlog.Llevel | xlog.Llongfile | xlog.Ltime)
	xlog := xlog.NewWith(context.Background())
	app := cli.App("gb28181Simulator", "Runs the gb28181 simulator.")
	app.Spec = "[ -c=<configuration path> ] "
	confPath := app.StringOpt("c config", "sim.conf", "Specifies the configuration path (file) to use for the simulator.")
	app.Action = func() { run(xlog, app, confPath) }

	// Register sub-commands
	app.Command("version", "Prints the version of the executable.", version.Print)
	app.Run(os.Args)
}

func run(xlog *xlog.Logger, app *cli.Cli, conf *string) {
	xlog.Infof("gb28181 simulator is running...")
	cfg, err := config.ParseJsonConfig(conf)
	if err != nil {
		xlog.Errorf("load config file failed, err = ", err)
	}
	xlog.Infof("config file = %#v", cfg)
	srv, err := useragent.NewService(xlog, cfg)
	if err != nil {
		xlog.Infof("new service failed err = %#v", err)
		return
	}
	srv.HandleIncommingMsg()
}

func mp4ConvertPsStream() {
	// https://www.likecs.com/show-205030086.html
	// https://www.cnblogs.com/dong1/p/11051708.html

	rtp := packet.NewRRtpTransfer("", packet.LocalCache, 0)

	// send ip,port and recv ip,port
	rtp.Service("127.0.0.1", "172.20.25.2", 10086, 10087)

	f, err := avutil.Open("1.mp4") // 要转换的视频文件，输出为 test.data
	if err != nil {
		fmt.Errorf("read file error(%v)", err)
		rtp.Exit()
		return
	}

	var pts uint64 = 0
	streams, _ := f.Streams()
	var vindex int8
	fmt.Println("----- file info -----")
	for i, stream := range streams {
		// 查找H264视频流,获得视频流索引
		if stream.Type() == av.H264 {
			vindex = int8(i)
			fmt.Println("video stream index: ", vindex)
			// break
		}
		if stream.Type().IsAudio() {
			astream := stream.(av.AudioCodecData)
			fmt.Println("audio info: ", astream.Type(), astream.SampleRate(), astream.SampleFormat(), astream.ChannelLayout())
		}
		if stream.Type().IsVideo() {
			vstream := stream.(av.VideoCodecData)
			fmt.Println("video info: ", vstream.Type(), vstream.Width(), vstream.Height())
		}
	}
	fmt.Println("---------------------")

	count := 0
	for i := 0; i < 10000; i++ {
		var pkt av.Packet
		var err error
		// 每次读取一帧
		if pkt, err = f.ReadPacket(); err != nil {
			fmt.Printf("read packet error(%v) %d", err, count)
			goto STOP
		}
		// 筛选出 视频流
		if pkt.Idx != vindex {
			continue
		}
		// fmt.Println("pkt: ", count, streams[pkt.Idx].Type(), "len: ", len(pkt.Data), "keyframe: ", pkt.IsKeyFrame, "pts: ", pts)
		count += 1
		// fmt.Println(string(pkt.Data))
		rtp.Send2data(pkt.Data, pkt.IsKeyFrame, pts)
		pts += 40 // 1秒钟25帧， 每帧40ms
		// time.Sleep(time.Millisecond * 40)
	}
STOP:
	f.Close()
	rtp.Exit()
	return

}
