package main

import (
	_ "embed"
	"os"

	"github.com/mrhaoxx/AutoPXE/pxe"
	"github.com/mrhaoxx/AutoPXE/pxe/ipxe"
	"github.com/mrhaoxx/AutoPXE/tftp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// readHandler is called when client starts file download from server

var DEFAULT_DISTRO_VER = ""

func main() {
	// use nil in place of handler to disable read or write operations

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	log.Info().Msg("Welcome to AutoPXE - A auto PXE server for clusters")

	server := tftp.NewServer()
	pxe := pxe.Server{
		RootfsPath:    "/pxe/rootfs/",
		DefaultDistro: DEFAULT_DISTRO_VER,
	}
	ipxe := ipxe.NewServer()

	server.Handlers = append(server.Handlers, ipxe)
	server.Handlers = append(server.Handlers, &pxe)

	err := server.ListenAndServe(":69")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start TFTP server")
	}
}
