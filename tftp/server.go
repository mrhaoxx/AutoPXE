package tftp

import (
	"io"

	"github.com/pin/tftp/v3"
	"github.com/rs/zerolog/log"
)

type Ctx struct {
	MacAddress string
	IP         string
	Arch       string

	Path string

	Resp io.ReaderFrom
}

type Ret uint8

const (
	RequestEnd Ret = iota
	RequestNext
)

type TFTPHandler interface {
	Handle(*Ctx) Ret
}

type Server struct {
	s *tftp.Server

	Handlers []TFTPHandler
}

func (s *Server) handle(filename string, rf io.ReaderFrom) error {

	ctx := &Ctx{
		IP:         rf.(tftp.OutgoingTransfer).RemoteAddr().IP.String(),
		MacAddress: "",
		Path:       filename,
		Resp:       rf,
	}

	for _, h := range s.Handlers {
		if h.Handle(ctx) == RequestEnd {
			break
		}
	}

	log.Info().Str("IP", ctx.IP).Str("MAC", ctx.MacAddress).Str("Path", ctx.Path).Msg("TFTP request")

	return nil
}

func NewServer() *Server {

	s := &Server{}

	s.s = tftp.NewServer(s.handle, nil)

	return s
}

func (s *Server) ListenAndServe(addr string) error {
	return s.s.ListenAndServe(addr)
}
