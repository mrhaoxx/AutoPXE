package pxe

import (
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

type ScannedKernel struct {
	Version    string
	KernelPath string
	InitrdPath string
}

type ScannedDistroVersion struct {
	Version    string
	Kernels    []ScannedKernel
	RootfsPath string
}

type ScannedDistro struct {
	Name string

	Versions []ScannedDistroVersion
}

// rootfs/{distro}/{version}
// --> /{kernel,initrd} as ""
// --> /boot/{kernel,initrd} as options
// initrd.img-6.1.0-25-amd64

func ScanRootfs(rootfs_path string) (list []ScannedDistro) {

	log.Debug().Msg("Scanning rootfs")

	root, err := os.Open(rootfs_path)
	if err != nil {
		log.Err(err).Msg("Failed to open rootfs dir")
		return nil
	}
	defer root.Close()

	distros, err := root.Readdir(-1)
	if err != nil {
		log.Err(err).Msg("Failed to read rootfs dir")
		return nil
	}

	// Print the list of files
	for _, distro := range distros {
		if distro.IsDir() {

			distrodata := ScannedDistro{
				Name: distro.Name(),
			}

			ddir := path.Join(rootfs_path, distro.Name())
			distroDir, err := os.Open(ddir)

			if err != nil {
				log.Err(err).Str("distro", distro.Name()).Msg("Failed to open distro dir")
				continue
			}

			defer distroDir.Close()

			versions, err := distroDir.Readdir(-1)

			if err != nil {
				log.Err(err).Str("distro", distro.Name()).Msg("Failed to read distro dir")
				continue
			}

			for _, version := range versions {
				if version.IsDir() { // now we are in /rootfs/{distro}/{version}

					bloc := path.Join(distro.Name(), version.Name())

					vdir := path.Join(ddir, version.Name())

					bootdir, err := os.Open(path.Join(vdir, "boot"))

					versiondata := ScannedDistroVersion{
						Version:    version.Name(),
						RootfsPath: vdir,
					}

					if err != nil {
						log.Err(err).Str("version", version.Name()).Str("distro", distro.Name()).Str("path", vdir).Msg("Failed to open boot dir")
						continue
					}
					defer bootdir.Close()

					boots, err := bootdir.Readdir(-1)

					if err != nil {
						log.Err(err).Str("version", version.Name()).Str("distro", distro.Name()).Str("path", vdir).Msg("Failed to read boot dir")
						continue
					}

					var kernels map[string]string = make(map[string]string)
					var initrds map[string]string = make(map[string]string)

					if distro.Name() == "debian" {
						kernels["latest"] = path.Join(bloc, "vmlinuz")
						initrds["latest"] = path.Join(bloc, "initrd.img")
					}

					for _, boot := range boots {
						if boot.IsDir() {
							log.Debug().Str("dir", boot.Name()).Msg("Ignoring directory")
						}

						// initrd.img-6.1.0-25-amd64 --> initrd 6.1.0-25-amd64

						switch {
						case strings.HasPrefix(boot.Name(), "vmlinuz"):
							regexp := regexp.MustCompile(`vmlinuz-(.*)`)
							vers := regexp.FindStringSubmatch(boot.Name())
							if len(vers) == 0 {
								log.Warn().Str("file", boot.Name()).Str("path", vdir).Msg("Failed to parse kernel version")
								continue
							}
							version := vers[1]
							kernels[version] = path.Join(bloc, "boot", boot.Name())
							log.Info().Str("kversion", version).Str("kernel", kernels[version]).Str("path", vdir).Str("distro", distro.Name()).Msg("Found kernel")
						case strings.HasPrefix(boot.Name(), "initrd.img"):
							regexp := regexp.MustCompile(`initrd.img-(.*)`)
							vers := regexp.FindStringSubmatch(boot.Name())
							if len(vers) == 0 {
								log.Warn().Str("file", boot.Name()).Str("path", vdir).Msg("Failed to parse initrd version")
								continue
							}
							version := vers[1]
							initrds[version] = path.Join(bloc, "boot", boot.Name())
							log.Info().Str("iversion", version).Str("initrd", initrds[version]).Str("path", vdir).Str("distro", distro.Name()).Msg("Found initrd")
						// initramfs-6.6.0-45.0.0.54.oe2409.x86_64.img --> initramfs 6.6.0-45.0.0.54.oe2409.x86_64
						// seen in openEuler
						case strings.HasPrefix(boot.Name(), "initramfs"):
							regexp := regexp.MustCompile(`initramfs-(.*)\.img`)
							vers := regexp.FindStringSubmatch(boot.Name())
							if len(vers) == 0 {
								log.Warn().Str("file", boot.Name()).Str("path", vdir).Msg("Failed to parse initramfs version")
								continue
							}
							version := vers[1]
							initrds[version] = path.Join(bloc, "boot", boot.Name())
							log.Info().Str("iversion", version).Str("initrd", initrds[version]).Str("path", vdir).Str("distro", distro.Name()).Msg("Found initramfs")
						default:
							log.Debug().Str("file", boot.Name()).Msg("Ignoring non-kernel/initrd file")
						}
					}

					kernelsdata := make([]ScannedKernel, 0, len(kernels))

					for version, kernel := range kernels {
						kernelsdata = append(kernelsdata, ScannedKernel{
							Version:    version,
							KernelPath: kernel,
							InitrdPath: initrds[version],
						})
					}

					versiondata.Kernels = kernelsdata

					distrodata.Versions = append(distrodata.Versions, versiondata)

				} else {
					log.Debug().Str("file", version.Name()).Msg("Ignoring non-directory")
				}

			}

			list = append(list, distrodata)

		} else {
			log.Debug().Str("file", distro.Name()).Msg("Ignoring non-directory")
		}
	}

	return list

}
