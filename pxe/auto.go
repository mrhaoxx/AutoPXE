package pxe

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/mrhaoxx/AutoPXE/pxe/ipxe"
	"github.com/mrhaoxx/AutoPXE/tftp"
	"github.com/rs/zerolog/log"
)

type Server struct {
	RootfsPath string

	DefaultDistro string
}

func (s *Server) Handle(ctx *tftp.Ctx) tftp.Ret {
	if strings.HasPrefix(ctx.Path, "autopxe-") {

		script := ipxe.IPXEScript{}

		oss := ScanRootfs(s.RootfsPath)
		menu := ipxe.Menu{
			Title:   "AutoPXE Boot Main Menu " + ctx.MacAddress + " " + ctx.IP,
			Id:      "start",
			Timeout: "8000",
		}

		for _, os := range oss {
			menu.AddItem(os.Name, os.Name, "")
		}

		menu.AddItem("Configure settings", "config", "")
		menu.AddItem("Drop to iPXE shell", "shell", "")
		menu.AddItem("Reboot computer", "reboot", "")
		menu.AddItem("Exit iPXE and continue BIOS boot", "exit", "")

		menu.PrintTo(&script)

		script.Append(`:shell
echo Type 'exit' to get the back to the menu
shell
goto start

:failed
echo Booting failed, dropping to shell
goto shell

:reboot
reboot

:exit
exit

:config
config
goto start

`)

		for _, val := range oss {
			smenu := ipxe.Menu{
				Title:   "Boot " + val.Name,
				Id:      val.Name,
				Timeout: "4000",
				Cancel:  "start",
			}

			for _, ver := range val.Versions {
				for _, kernel := range ver.Kernels {
					smenu.AddItem("Boot "+val.Name+"-"+ver.Version+"-"+kernel.Version, val.Name+"-"+ver.Version+"-"+kernel.Version, "")
				}
			}

			smenu.AddItem("Back to main menu", "start", "")
			smenu.PrintTo(&script)
		}

		for _, val := range oss {
			for _, ver := range val.Versions {
				for _, kernel := range ver.Kernels {
					script.Append(":" + val.Name + "-" + ver.Version + "-" + kernel.Version + "\n")
					script.Append("kernel boot/" + kernel.KernelPath + "\n")
					script.Append("initrd boot/" + kernel.InitrdPath + "\n")
					script.Append("boot\n")
				}
			}
		}

		ctx.Resp.ReadFrom(strings.NewReader(script.Script))

		fmt.Println(script.Script)
		return tftp.RequestEnd
	} else if strings.HasPrefix(ctx.Path, "boot/") {
		file, err := os.Open(path.Join(s.RootfsPath, strings.TrimPrefix(ctx.Path, "boot")))
		if err != nil {
			log.Err(err).Msg("Failed to open file")
			return tftp.RequestEnd
		}
		defer file.Close()
		_, err = ctx.Resp.ReadFrom(file)
		if err != nil {
			log.Err(err).Msg("Failed to read file")
			return tftp.RequestEnd
		}
	}

	return tftp.RequestNext

}
