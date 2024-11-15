package ipxe

import (
	"bytes"
	_ "embed"
	"strings"

	"github.com/mrhaoxx/AutoPXE/tftp"
)

//go:embed snponly.efi
var ipxe_efi_x86 []byte

//go:embed undionly.kpxe
var ipxe_bios_x86 []byte

type IPXEServer struct {
}

func NewServer() *IPXEServer {
	return &IPXEServer{}
}

func (s *IPXEServer) Handle(ctx *tftp.Ctx) tftp.Ret {

	switch ctx.Path {
	case "undionly.kpxe":
		ctx.Resp.ReadFrom(bytes.NewReader(ipxe_bios_x86))
		return tftp.RequestEnd
	case "ipxe.efi":
		ctx.Resp.ReadFrom(bytes.NewReader(ipxe_efi_x86))
		return tftp.RequestEnd
	default:
		if strings.HasPrefix(ctx.Path, "autopxe-") {
			// autopxe-{mac}
			mac := strings.TrimPrefix(ctx.Path, "autopxe-")
			ctx.MacAddress = mac
			return tftp.RequestNext
		}
	}
	return tftp.RequestNext
}
