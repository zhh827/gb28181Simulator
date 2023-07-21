package message

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
)

var (
	version = "yunli/0.3"
)

func Version() string {
	return version
}
func Print(cli *cli.Cmd) {
	fmt.Println("version = ", Version())
}
