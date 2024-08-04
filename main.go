package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "embed"

	"github.com/pin/tftp/v3"
)

//go:embed ipxe.efi
var ipxe_efi []byte

//go:embed undionly.kpxe
var ipxe_bios []byte

// from /rootfs/debian/bookworm/boot

type BootableRootfs struct {
	Title string

	Rootfs string

	Version string
	Distro  string

	Kversion string

	Kernel string
	Initrd string

	UUID string

	BootOptions string
	Tag         string
}

func Hash(strs ...string) string {
	// 将所有输入字符串连接成一个单独的字符串
	concatenated := strings.Join(strs, "")
	// 创建一个新的哈希器
	hasher := sha256.New()
	// 写入要哈希处理的字符串
	hasher.Write([]byte(concatenated))
	// 计算哈希值并将结果转换为十六进制字符串
	hashed := hasher.Sum(nil)
	return hex.EncodeToString(hashed)
}

func ScanRootfs() (list []BootableRootfs) {
	log.Println("Scanning rootfs")

	root, err := os.Open("/rootfs")
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer root.Close()

	// Get the list of distros
	distros, err := root.Readdir(-1)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// Print the list of files
	for _, distro := range distros {
		if distro.IsDir() {
			fmt.Println(distro.Name(), ":")
			distroDir, err := os.Open("/rootfs/" + distro.Name())
			if err != nil {
				fmt.Println(err)
				return nil
			}
			defer distroDir.Close()
			versions, err := distroDir.Readdir(-1)

			if err != nil {
				fmt.Println(err)
				return nil
			}

			for _, version := range versions {
				if version.IsDir() {

					fmt.Println("  ", version.Name(), ": ")
					// Open the version directory
					versionDir, err := os.Open("/rootfs/" + distro.Name() + "/" + version.Name() + "/boot")
					if err != nil {
						fmt.Println(err)
						continue
					}
					defer versionDir.Close()

					boots, err := versionDir.Readdir(-1)

					if err != nil {
						fmt.Println(err)
						continue
					}

					var kernels map[string]string = make(map[string]string)
					var initrds map[string]string = make(map[string]string)

					for _, boot := range boots {
						fmt.Print("		", boot.Name())
						switch {
						case strings.HasPrefix(boot.Name(), "vmlinuz"):
							re := regexp.MustCompile(`vmlinuz(.*)`)
							version := re.FindStringSubmatch(boot.Name())
							if version[1] == ".old" {
								fmt.Println(": ignored")
								continue
							}

							fmt.Println(": kernel=", version)
							kernels[version[1]] = boot.Name()
						case strings.HasPrefix(boot.Name(), "initrd.img"):
							re := regexp.MustCompile(`initrd.img(.*)`)
							version := re.FindStringSubmatch(boot.Name())
							if version[1] == ".old" {
								fmt.Println(": ignored")
								continue
							}
							fmt.Println(": initrd=", version)

							initrds[version[1]] = boot.Name()
						case strings.HasPrefix(boot.Name(), "initramfs"):
							re := regexp.MustCompile(`initramfs(.*)\.img`)
							version := re.FindStringSubmatch(boot.Name())
							if version[1] == ".old" {
								fmt.Println(": ignored")
								continue
							}
							fmt.Println(": initramfs=", version)

							initrds[version[1]] = boot.Name()
						default:
							fmt.Println(": ignored")
						}
					}

					for _version, kernel := range kernels {
						if initrd, ok := initrds[_version]; ok {
							Add := func(Tag, Title string, Option string) {
								list = append(list, BootableRootfs{
									Rootfs: "/pxe/rootfs/" + distro.Name() + "/" + version.Name(),

									Title: distro.Name() + " " + version.Name() + " " + _version + " " + Title,

									Kernel: "boot/" + distro.Name() + "/" + version.Name() + "/boot/" + kernel,
									Initrd: "boot/" + distro.Name() + "/" + version.Name() + "/boot/" + initrd,

									Distro:      distro.Name(),
									Version:     version.Name(),
									Kversion:    _version,
									BootOptions: Option,
									Tag:         Tag,

									UUID: Hash(distro.Name(), version.Name(), _version, Option),
								})
							}
							Add("nfsrw", "NFS RW", "ip=dhcp rw")
							Add("ovl", "OverlayFS", "ip=dhcp rootovl")
						}
					}

				}
			}
		}
	}

	return list

}

// readHandler is called when client starts file download from server
func readHandler(filename string, rf io.ReaderFrom) error {
	fmt.Printf("reading: %s\n", filename)

	switch filename {
	case "ipxe.efi":
		rf.ReadFrom(bytes.NewReader(ipxe_efi))
		return nil
	case "undionly.kpxe":
		rf.ReadFrom(bytes.NewReader(ipxe_bios))
		return nil
	default:
		if strings.HasPrefix(filename, "autopxe-") {
			mac := strings.TrimPrefix(filename, "autopxe-")

			rootfs := ScanRootfs()
			script := `#!ipxe
dhcp
`

			var def map[string]BootableRootfs = make(map[string]BootableRootfs)

			sort.Slice(rootfs, func(i, j int) bool {
				return strings.Compare(rootfs[i].Distro+rootfs[i].Version+rootfs[i].Kversion, rootfs[i].Distro+rootfs[i].Version+rootfs[i].Kversion) < 0
			})

			for _, rootfs := range rootfs {
				if rootfs.Kversion == "" {
					def[rootfs.Distro+"-"+rootfs.Version] = rootfs
					def[rootfs.Distro+"-"+rootfs.Version+"--"+rootfs.Tag] = rootfs
				} else {
					def[rootfs.Distro+"-"+rootfs.Version+"-"+rootfs.Kversion] = rootfs
					def[rootfs.Distro+"-"+rootfs.Version+"-"+rootfs.Kversion+"-"+rootfs.Tag] = rootfs
				}

			}

			for k, rootfs := range def {
				fmt.Println(k, ":")
				fmt.Println("	", rootfs.Title)
			}

			if DEFAULT_DISTRO_VER != "" {
				if _, ok := def[DEFAULT_DISTRO_VER]; ok {
					script += fmt.Sprintf("set menu-default %s\n", def[DEFAULT_DISTRO_VER].UUID)
				}
			}

			// Generate iPXE script
			script += `
set menu-timeout 15000
set nfs-server 172.25.2.10

:start
menu iPXE boot menu for ` + mac + `
item --gap --             Welcome to AutoPXE, ` + mac + `!
item --key b boot         Boot from local disk
item --gap --             ------------------------- Operating systems ------------------------------
`

			for _, rootfs := range rootfs {
				script += fmt.Sprintf("item %s %s\n", rootfs.UUID, rootfs.Title)
			}

			script +=
				`
item --gap --             ------------------------- Advanced options -------------------------------
item --key c config       Configure settings
item shell                Drop to iPXE shell
item reboot               Reboot computer
item
item --key x exit         Exit iPXE and continue BIOS boot
choose --timeout ${menu-timeout} --default ${menu-default} selected || goto cancel
set menu-timeout 0
goto ${selected}

:cancel
echo You cancelled the menu, dropping you to a shell

:shell
echo Type 'exit' to get the back to the menu
shell
set menu-timeout 0
set submenu-timeout 0
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

:back
set submenu-timeout 0
clear submenu-default
goto start

`
			for _, rootfs := range rootfs {
				script += fmt.Sprintf(`:%s
echo Booting %s
initrd %s
chain %s root=/dev/nfs nfsroot=${nfs-server}:%s %s
shell
boot || goto failed
goto start
`, rootfs.UUID, rootfs.Distro+" "+rootfs.Version+" "+rootfs.Kversion,
					rootfs.Initrd, rootfs.Kernel, rootfs.Rootfs, rootfs.BootOptions)
			}

			fmt.Println(script)
			rf.ReadFrom(strings.NewReader(script))
			return nil
		} else if strings.HasPrefix(filename, "boot/") {
			file, err := os.Open("/rootfs/" + strings.TrimPrefix(filename, "boot/"))
			if err != nil {
				fmt.Println(err)
				return err
			}
			defer file.Close()

			_, err = rf.ReadFrom(file)

			if err != nil {
				fmt.Println(err)
				return err
			}
		}

	}

	return nil
}

var DEFAULT_DISTRO_VER = ""

func main() {
	// use nil in place of handler to disable read or write operations

	DEFAULT_DISTRO_VER, _ = os.LookupEnv("DEFAULT_DISTRO_VER")

	fmt.Println("DEFAULT_DISTRO_VER=", DEFAULT_DISTRO_VER)

	s := tftp.NewServer(readHandler, nil)
	s.SetTimeout(5 * time.Second)  // optional
	err := s.ListenAndServe(":69") // blocks until s.Shutdown() is called
	if err != nil {
		fmt.Fprintf(os.Stdout, "server: %v\n", err)
		os.Exit(1)
	}
}
