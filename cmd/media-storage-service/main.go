package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/AxisCommunications/body-worn-integration-api/server"

	"github.com/kardianos/service"
)

var (
	logger  service.Logger
	exePath string
	//following vars are set by the linker
	version   string = "development"
	buildTime string
)

func init() {
	ex, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exePath = filepath.Dir(ex)
}

// struct for running server as service, implements Interface from kardianos/service
type mediaStorageService struct {
	exit chan struct{}
}

func (m *mediaStorageService) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	m.run()
	return nil
}

func (m *mediaStorageService) run() {
	m.exit = make(chan struct{})
	server.SetLogger(logger)
	s, err := server.New(exePath)
	if err != nil {
		log.Println("No settings found, run with install flag to create and configure as service.")
		log.Fatalf("Failed to start server: %v", err)
	}
	logger.Info("Starting Axis body worn Swift service example: " + version)
	logger.Info("Built on " + buildTime)
	go s.Run(m.exit)
}

func (m *mediaStorageService) Stop(s service.Service) error {
	logger.Info("Shutting down")
	close(m.exit)
	return nil
}

func main() {

	svcConfig := &service.Config{
		Name:        "AxisBodyWornSwiftServiceExample",
		DisplayName: "Axis body worn Swift service example",
		Description: "Axis body worn Swift service example is a simple blob storage implementing the subsection of the Swift API supported by Axis BWS.",
	}

	mss := &mediaStorageService{}
	s, err := service.New(mss, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "help", "--help", "-h":
			printUsage()

		case "install":
			fmt.Println("Installing Axis body worn Swift service example version: " + version)
			fmt.Println("Built on " + buildTime)
			err = server.Configure(exePath, version)
			if err != nil {
				log.Fatalf("Error configuring service %v", err)
			}
			fallthrough // service.Control below needs to run after the installer

		case "start", "stop", "restart", "uninstall":
			// this will run on "install" too.
			err = service.Control(s, os.Args[1])
			if err != nil {
				log.Fatal(err)
			}

		default:
			fmt.Printf("%q is an unknown command. Type '%s help' for help.\n", os.Args[1], os.Args[0])
		}
		return
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

func printUsage() {
	fmt.Println(`Axis body worn Swift service example usage

Arguments
  help		Show this message.
  install	Enter install dialog to generate a connection config and install
  		as a service.
  uninstall 	Uninstall service.
  start		Start the service.
  stop		Stop the service.

config.json
  This is the connection file you upload to a system controller in order to
  connect to this instance of the Axis body worn Swift service example.
  It will be generated during installation and can be found in the storage
  location entered during installation.

settings.cfg
  Is generated during installation in the same folder as the executable. It
  holds information used to run the service.

Encryped files
  To enable content encryption you need to supply a public RSA keyfile (PEM
  encoded, >=1024 bit) during the installation. The filename will be used as the
  key ID.
  You can generate an RSA keypair using this command:

    $ openssl genpkey -algorithm RSA -out private_key.pem -pkeyopt rsa_keygen_bits:2048

  Then extract the public key:

    $ openssl rsa -pubout -in private_key.pem -out public_key.pem

  Note that on Windows and some Linux distros you will need to install OpenSSL
  first. If you run the command where you don't have write permission you won't
  get an ouput file. To solve this on Windows you could add "C:\" to the start
  of the -out and -in paths.

GNSS files
  To view GNSS files that has been created with Open-API default format run the
  seperate executable 'GNSSViewerExample'.

  ./GNSSViewerExample <path to gnss file>
	`)
}
