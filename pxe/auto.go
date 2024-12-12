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

	Env map[string]string

	CmdlineTemplates map[string]string

	HostDefaults map[string]string

	DefaultDistroPattern map[string][]ScannedBootFile
}

func (s *Server) Handle(ctx *tftp.Ctx) tftp.Ret {
	if strings.HasPrefix(ctx.Path, "autopxe-") {

		defaultd := s.DefaultDistro
		if s.HostDefaults != nil {
			for k, val := range s.HostDefaults {
				if strings.EqualFold(k, ctx.MacAddress) {
					defaultd = val
					break
				}
			}
		}

		script := ipxe.IPXEScript{}

		script.Append("#!ipxe\n")

		if s.Env != nil {
			for k, v := range s.Env {
				script.Set(k, v)
			}
		}

		distros := Scanner{DefaultDistroPattern: s.DefaultDistroPattern}.ScanRootfs(s.RootfsPath)

		menu := ipxe.Menu{
			Title:   "AutoPXE Boot Main Menu " + ctx.MacAddress + " " + ctx.IP + " ${hostname}",
			Id:      "start",
			Timeout: "10000",
		}

		// check if the default distro is in the list

		menu.AddItem("Boot "+defaultd, defaultd, "")

		for _, distro := range distros {
			menu.AddItem(distro.Name, distro.Name, "")
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

		for _, distro := range distros {
			smenu := ipxe.Menu{
				Title:  "Boot " + distro.Name,
				Id:     distro.Name,
				Cancel: "start",
			}

			for _, release := range distro.Release {
				for _, bootfile := range release.BootFiles {
					for k := range s.CmdlineTemplates {
						smenu.AddItem("Boot "+distro.Name+"/"+release.Release+"/"+bootfile.Version.Raw+"/"+k, distro.Name+"/"+release.Release+"/"+bootfile.Version.Raw+"/"+k, "")
					}
				}
			}

			smenu.AddItem("Back to main menu", "start", "")
			smenu.PrintTo(&script)
		}

		for _, val := range distros {
			for _, ver := range val.Release {
				for _, kernel := range ver.BootFiles {
					for k, v := range s.CmdlineTemplates {
						script.Label(val.Name + "/" + ver.Release + "/" + kernel.Version.Raw + "/" + k)
						script.Set("rootfs-path", ver.RootfsPath)
						script.Echo("Booting " + val.Name + "/" + ver.Release + "/" + kernel.Version.Raw + "/" + k + "\n")
						script.Echo("Cmdline: " + v + "\n")
						script.Append("initrd boot/" + kernel.InitrdPath + "\n")

						script.Append("imgstat\n")

						script.Echo("Booting in 3 seconds...")
						script.Append("sleep 3\n")

						script.Append("chain boot/" + kernel.KernelPath + " " + v + "\n")
						script.Append("boot || goto failed\n")
					}
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
