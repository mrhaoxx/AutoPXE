package pxe

import (
	"fmt"

	"github.com/mrhaoxx/AutoPXE/pxe/ipxe"
)

type LinuxBootable struct {
	Id      string
	Initrd  string // initrd --name initramfs %s
	Kernel  string //
	Cmdline string // chain %s %s
}

func (l *LinuxBootable) PrintTo(script *ipxe.IPXEScript) {
	script.Append(fmt.Sprintf(":%s\n", l.Id))
	script.Append(fmt.Sprintf("initrd %s\n", l.Initrd))
	script.Append(fmt.Sprintf("chain %s %s\n", l.Kernel, l.Cmdline))
	script.Append("shell\n")
}
