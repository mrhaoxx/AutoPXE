package pxe

import (
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// KernelVersion represents a kernel version
// format: MAJOR.MINOR.PATCH-SUFFIX
type KernelVersion struct {
	Numbers []int
	Suffix  string
	Raw     string
}

func NewKernelVersion(versionStr string) KernelVersion {
	parts := strings.SplitN(versionStr, "-", 2)
	numericPart := strings.Split(parts[0], ".")
	var nums []int

	for _, n := range numericPart {
		i, err := strconv.Atoi(n)
		if err != nil {
			i = 0
		}
		nums = append(nums, i)
	}

	return KernelVersion{
		Numbers: nums,
		Suffix:  parts[1],
		Raw:     versionStr,
	}
}

func (kv KernelVersion) Compare(other KernelVersion) bool {
	for i := 0; i < len(kv.Numbers) || i < len(other.Numbers); i++ {
		n1, n2 := 0, 0
		if i < len(kv.Numbers) {
			n1 = kv.Numbers[i]
		}
		if i < len(other.Numbers) {
			n2 = other.Numbers[i]
		}
		if n1 != n2 {
			return n1 < n2
		}
	}

	return kv.Suffix < other.Suffix
}

// ScannedBootFile represents a kernel with its corresponding initrd/initramfs
type ScannedBootFile struct {
	Version    KernelVersion
	KernelPath string
	InitrdPath string
}

func (sbf ScannedBootFile) Compare(other ScannedBootFile) bool {
	return sbf.Version.Compare(other.Version)
}

type ScannedBootFileSlice []ScannedBootFile

func (s ScannedBootFileSlice) Len() int {
	return len(s)
}

func (s ScannedBootFileSlice) Less(i, j int) bool {
	return s[i].Compare(s[j])
}

func (s ScannedBootFileSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type ScannedRelease struct {
	Release    string
	BootFiles  []ScannedBootFile
	RootfsPath string
}

type ScannedDistro struct {
	Name    string
	Release []ScannedRelease
}

// ScanRootfs scans the rootfs directory for distros, releases and kernels
// Path should be like: rootfs/{distro}/{release}/boot/{vmlinuz,initrd|initramfs}
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
		if !distro.IsDir() {
			log.Debug().Str("file", distro.Name()).Msg("Ignoring non-directory")
			continue
		}

		distroPath := path.Join(rootfs_path, distro.Name())
		distroDir, err := os.Open(distroPath)
		if err != nil {
			log.Err(err).Str("distro", distro.Name()).Msg("Failed to open distro dir")
			continue
		}
		defer distroDir.Close()

		// now we are in /rootfs/{distro}
		distroData := ScannedDistro{
			Name: distro.Name(),
		}

		releases, err := distroDir.Readdir(-1)
		if err != nil {
			log.Err(err).Str("distro", distro.Name()).Msg("Failed to read distro dir")
			continue
		}

		for _, release := range releases {
			if !release.IsDir() {
				log.Debug().Str("file", release.Name()).Msg("Ignoring non-directory")
				continue
			}
			releasePath := path.Join(distroPath, release.Name())

			releaseData := ScannedRelease{
				Release:    release.Name(),
				RootfsPath: releasePath,
			}

			bootPath := path.Join(distro.Name(), release.Name())
			bootDir, err := os.Open(path.Join(releasePath, "boot"))
			if err != nil {
				log.Err(err).Str("version", release.Name()).Str("distro", distro.Name()).Str("path", releasePath).Msg("Failed to open boot dir")
				continue
			}
			defer bootDir.Close()

			// now we are in /rootfs/{distro}/{release}/boot
			bootFiles, err := bootDir.Readdir(-1)
			if err != nil {
				log.Err(err).Str("version", release.Name()).Str("distro", distro.Name()).Str("path", releasePath).Msg("Failed to read boot dir")
				continue
			}

			var kernels map[string]string = make(map[string]string)
			var initrds map[string]string = make(map[string]string)

			for _, bootfile := range bootFiles {
				if bootfile.IsDir() {
					log.Debug().Str("dir", bootfile.Name()).Msg("Ignoring directory")
					continue
				}

				switch {
				case strings.HasPrefix(bootfile.Name(), "vmlinuz"):
					// vmlinuz-5.10.0-8-amd64 --> vmlinuz 5.10.0-8-amd64
					regexp := regexp.MustCompile(`vmlinuz-(.*)`)
					vers := regexp.FindStringSubmatch(bootfile.Name())
					if len(vers) == 0 {
						log.Warn().Str("file", bootfile.Name()).Str("path", releasePath).Msg("Failed to parse kernel version")
						continue
					}
					version := vers[1]
					kernels[version] = path.Join(bootPath, "boot", bootfile.Name())
					log.Info().Str("kversion", version).Str("kernel", kernels[version]).Str("path", releasePath).Str("distro", distro.Name()).Msg("Found kernel")
				case strings.HasPrefix(bootfile.Name(), "initrd.img"):
					// initrd.img-6.1.0-25-amd64 --> initrd 6.1.0-25-amd64
					regexp := regexp.MustCompile(`initrd.img-(.*)`)
					vers := regexp.FindStringSubmatch(bootfile.Name())
					if len(vers) == 0 {
						log.Warn().Str("file", bootfile.Name()).Str("path", releasePath).Msg("Failed to parse initrd version")
						continue
					}
					version := vers[1]
					initrds[version] = path.Join(bootPath, "boot", bootfile.Name())
					log.Info().Str("iversion", version).Str("initrd", initrds[version]).Str("path", releasePath).Str("distro", distro.Name()).Msg("Found initrd")
				case strings.HasPrefix(bootfile.Name(), "initramfs"):
					// initramfs-6.6.0-45.0.0.54.oe2409.x86_64.img --> initramfs 6.6.0-45.0.0.54.oe2409.x86_64
					regexp := regexp.MustCompile(`initramfs-(.*)\.img`)
					vers := regexp.FindStringSubmatch(bootfile.Name())
					if len(vers) == 0 {
						log.Warn().Str("file", bootfile.Name()).Str("path", releasePath).Msg("Failed to parse initramfs version")
						continue
					}
					version := vers[1]
					initrds[version] = path.Join(bootPath, "boot", bootfile.Name())
					log.Info().Str("iversion", version).Str("initrd", initrds[version]).Str("path", releasePath).Str("distro", distro.Name()).Msg("Found initramfs")
				default:
					log.Debug().Str("file", bootfile.Name()).Msg("Ignoring non-kernel/initrd file")
				}
			}

			bootFileData := make([]ScannedBootFile, 0, len(kernels))

			for version, kernel := range kernels {
				initrd, ok := initrds[version]
				if !ok {
					log.Warn().Str("version", version).Str("kernel", kernel).Msg("No matching initrd found for kernel, ignoring")
					continue
				}
				bootFileData = append(bootFileData, ScannedBootFile{
					Version:    NewKernelVersion(version),
					KernelPath: kernel,
					InitrdPath: initrd,
				})
			}

			// duplicate a special latest version
			if len(bootFileData) > 0 {
				sort.Sort(ScannedBootFileSlice(bootFileData))
				latest := bootFileData[len(bootFileData)-1]
				latest.Version.Raw = "latest"
				bootFileData = append([]ScannedBootFile{latest}, bootFileData...)
			} else {
				log.Warn().Str("distro", distroData.Name).Str("release", releaseData.Release).Msg("No boot files found")
				continue
			}

			releaseData.BootFiles = bootFileData
			distroData.Release = append(distroData.Release, releaseData)
		}

		if len(distroData.Release) == 0 {
			log.Warn().Str("distro", distroData.Name).Msg("No releases found")
			continue
		}

		list = append(list, distroData)

	}

	return list

}
