package main

import (
	"context"
	"my28181/config"
	"my28181/message"

	"github.com/qiniu/x/xlog"
)

func main() {
	xlog.SetOutputLevel(0)
	xlog.SetFlags(xlog.Llevel | xlog.Llongfile | xlog.Ltime)
	xlog := xlog.NewWith(context.Background())
	run(xlog, "sim.conf")
}

func run(xlog *xlog.Logger, conf string) {
	xlog.Infof("gb28181 simulator is running...")
	cfg, err := config.ParseJsonConfig(conf)
	if err != nil {
		xlog.Errorf("加载配置文件错误: %s", err)
	}
	// xlog.Infof("config file = %#v", cfg)
	srv, err := message.NewService(xlog, cfg)
	if err != nil {
		xlog.Infof("new service failed err = %#v", err)
		return
	}
	srv.HandleIncommingMsg()
}
