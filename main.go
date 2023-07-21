package main

import (
	"context"
	"os"

	"my28181/config"
	"my28181/message/version"

	"my28181/message"

	cli "github.com/jawher/mow.cli"
	"github.com/qiniu/x/xlog"
)

func main() {
	//xlog := xlog.NewWith(context.Background())
	xlog.SetOutputLevel(0)
	xlog.SetFlags(xlog.Llevel | xlog.Llongfile | xlog.Ltime)
	xlog := xlog.NewWith(context.Background())
	app := cli.App("gb28181Simulator", "Runs the gb28181 simulator.")
	app.Spec = "[ -c=<configuration path> ] "
	confPath := app.StringOpt("c config", "sim.conf", "配置文件路径")
	app.Action = func() { run(xlog, app, confPath) }

	// Register sub-commands
	app.Command("version", "版本号", version.Print)
	app.Run(os.Args)
}

func run(xlog *xlog.Logger, app *cli.Cli, conf *string) {
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
