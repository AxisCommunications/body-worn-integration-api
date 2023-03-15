package main

import (
	"bufio"
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	logger  service.Logger
	exePath string
	//following vars are set by the linker
	version   string = "development"
	buildTime string
)

const configFilename = "settings.cfg"
const certFilename = "server.crt"
const keyFilename = "server.key"

// struct for running server as service, implements Interface from kardianos/service
type mss struct{}

func (m *mss) Start(s service.Service) error {
	// Start should not block. Do the actual work async.
	m.run()
	return nil
}

func (m *mss) run() {

	conf := Settings{}

	confFile, err := os.Open(filepath.Join(exePath, configFilename))
	if err != nil {
		log.Println("No settings found, run with install flag to create and configure as service.")
		log.Fatal(err)
	}
	defer confFile.Close()
	confBytes, err := io.ReadAll(confFile)
	if err != nil {
		log.Fatal(err)
	}
	err = json.Unmarshal(confBytes, &conf)
	if err != nil {
		log.Fatal(err)
	}

	server := NewServer(&conf)
	go server.Run()
}

//TODO do a proper shutdown
func (m *mss) Stop(s service.Service) error {
	return nil
}

func main() {
	ex, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exePath = filepath.Dir(ex)

	svcConfig := &service.Config{
		Name:        "AxisBodyWornSwiftServiceExample",
		DisplayName: "Axis body worn Swift service example",
		Description: "Axis body worn Swift service example is a simple blob storage implementing the subsection of the Swift API supported by Axis BWS.",
	}

	server := &mss{}
	s, err := service.New(server, svcConfig)
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
			err = configure()
			if err != nil {
				log.Fatal(err)
			}
			fallthrough // service.Control below needs to run after the installer

		case "start", "stop", "restart", "uninstall":
			// this will run on "install" too.
			err = service.Control(s, os.Args[1])
			if err != nil {
				log.Fatal(err)
			}

		default:
			fmt.Printf("%s is an unknown command. Type '%s help' for help.\n", os.Args[1], os.Args[0])
		}
		return
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

func getHostIPs() ([]string, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	fmt.Println("Choose which IP(s) the server should run on >")
	ips := []string{}
	iprev := 0
	i := 0
	for _, iface := range list {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		iprev = i
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
					i++
				}
			}
		}
		if i > iprev {
			fmt.Println(iface.Name)
			for i > iprev {
				fmt.Printf("%d: %s\n", iprev, ips[iprev])
				iprev++
			}
		}
	}
	fmt.Printf("%d: choose all.\n", i)
	fmt.Printf("%d: enter an ip.\n", i+1)
	var choice string
	for {
		fmt.Printf("Choose 0 - %d\n", i+1)
		fmt.Scanln(&choice)
		n, err := strconv.Atoi(choice)
		if err != nil {
			continue
		}
		if n == i {
			return ips, nil // all IPs chosen
		}
		if n < i && n >= 0 {
			return []string{ips[n]}, nil // nth IP chosen
		}
		if n != i+1 {
			continue
		}
		for {
			var ip string
			fmt.Println("Enter ip:")
			fmt.Scanln(&ip)
			tmp := net.ParseIP(ip)
			if tmp != nil {
				return []string{ip}, nil
			}
			fmt.Println("Invalid ip, please try again..")
		}

	}
}

func yesNoQuestion(question string) bool {
	var answer string
	for {
		fmt.Println(question)
		fmt.Scanln(&answer)
		resp := strings.ToLower(answer)
		if resp == "y" || resp == "yes" {
			return true
		}
		if resp == "n" || resp == "no" {
			return false
		}
	}
}

func pemBlockForKey(priv crypto.Signer) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

func buildCertName(i int) string {
	return fmt.Sprintf("%d_%s", i, certFilename)
}

func buildKeyName(i int) string {
	return fmt.Sprintf("%d_%s", i, keyFilename)
}

// Return all IPs that successfully got a cert generated.
func generateCerts(ips []string) []string {
	i := 0
	successIPs := []string{}
	for _, ip := range ips {
		err := generateCert(ip, buildCertName(i), buildKeyName(i))
		if err != nil {
			fmt.Printf("Failed to generate certificate for ip %s: %v", ip, err)
			continue
		}
		successIPs = append(successIPs, ip)
		i++
	}
	return successIPs
}

func generateCert(ip, certFilename, keyFilename string) error {
	// priv, err := rsa.GenerateKey(rand.Reader, *rsaBits)
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		return err
	}

	ipnet, _, err := net.ParseCIDR(ip + "/24")
	if err != nil {
		return errors.New("Error couldn't parse the ip address")
	}

	ips := []net.IP{ipnet}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Generated Co"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 360),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return err
	}
	out := &bytes.Buffer{}
	pem.Encode(out, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	err = os.WriteFile(filepath.Join(exePath, certFilename), []byte(out.String()), 0644)
	if err != nil {
		return err
	}
	out.Reset()
	block := pemBlockForKey(priv)
	if block == nil {
		return errors.New("Error generating a pem block, failed to generate certificate")
	}
	pem.Encode(out, pemBlockForKey(priv))
	err = os.WriteFile(filepath.Join(exePath, keyFilename), []byte(out.String()), 0644)
	if err != nil {
		return err
	}
	return nil
}

func selectPassword() (plaintext string, hash []byte) {
	for {
		fmt.Println("Select a password >")
		password, _ := terminal.ReadPassword(int(syscall.Stdin))

		fmt.Println("Re-enter password >")
		repeat, _ := terminal.ReadPassword(int(syscall.Stdin))

		if len(password) == 0 {
			fmt.Println("Password can not be empty.")
			continue
		}

		if bytes.Compare(password, repeat) != 0 {
			fmt.Println("Passwords do not match.")
			continue
		}

		// Hash and salt password
		hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Unable to create password digest: %v\nTry again.\n", err)
			continue
		}

		return string(password), hash
	}
}

func testPublicKey(bytes []byte) error {
	block, _ := pem.Decode(bytes)
	if block == nil {
		return errors.New("PEM parser error")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return errors.New("DER parser error")
	}
	switch pub.(type) {
	case *rsa.PublicKey:
		return nil
	}
	return errors.New("unknown type of public key")
}

// useContentEncryption returns key as an empty string if no keyfile is chosen,
// representing wanting no encryption, or a base64 encoded public key.
// Expects a PEM encoded >=1024 bit RSA key.
// keyID uses the keyfile filename as ID currently, but it could be anything.
func useContentEncryption() (key, keyID string) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Println("Enter the path to a public RSA keyfile. See help for details.\nLeave empty to not use existing encryption key >")
		scanner.Scan()
		publicKeyLocation := scanner.Text()

		if publicKeyLocation == "" {
			break // Break to ask if we should generate the keys
		}

		key, keyID, err := readPubkey(publicKeyLocation)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		return key, keyID
	}

	for {
		fmt.Println("Enter output directory to generate new content encryption keys.\nLeave empty to not use content encryption >")
		scanner.Scan()
		keyDir := scanner.Text()
		if keyDir == "" {
			return "", ""
		}

		privKeyPath, pubKeyPath, err := genContentEncryptionKeys(keyDir)
		if err != nil {
			fmt.Printf("Error generating keys: %v\n", err.Error())
			continue
		}
		fmt.Printf("Write key files to %q and %q\n", privKeyPath, pubKeyPath)
		key, keyID, err := readPubkey(pubKeyPath)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		return key, keyID
	}
}

func readPubkey(pubKeyFile string) (key, keyID string, err error) {

	file, err := os.Open(pubKeyFile)
	if err != nil {
		return "", "", fmt.Errorf("Could not load keyfile: %v\n", err)
	}
	b, err := io.ReadAll(file)
	file.Close()
	if err != nil {
		return "", "", fmt.Errorf("Could not read keyfile: %v\n", err)
	}
	err = testPublicKey(b)
	if err != nil {
		return "", "", fmt.Errorf("Invalid key format: %v\n", err)
	}

	key = base64.StdEncoding.EncodeToString(b)
	keyID = strings.TrimSuffix(filepath.Base(pubKeyFile), filepath.Ext(pubKeyFile))
	return key, keyID, nil
}

func genContentEncryptionKeys(keyDir string) (privKeyPath, pubKeyPath string, err error) {
	fi, err := os.Stat(keyDir)
	if err != nil && !os.IsNotExist(err) {
		return "", "", fmt.Errorf("unable to read dir %q", keyDir)
	}
	if fi == nil {
		err := os.MkdirAll(keyDir, 0755)
		if err != nil {
			return "", "", fmt.Errorf("could not create dir %q", keyDir)
		}
	}
	if fi != nil && !fi.Mode().IsDir() {
		return "", "", fmt.Errorf("%q is not a directory", keyDir)
	}

	pubKeyPath = filepath.Join(keyDir, "contentkey.public.pem")
	privKeyPath = filepath.Join(keyDir, "contentkey.private.pem")
	_, err = os.Stat(pubKeyPath)
	if err == nil {
		return "", "", fmt.Errorf("file already exists %q", pubKeyPath)
	}
	_, err = os.Stat(privKeyPath)
	if err == nil {
		return "", "", fmt.Errorf("file already exists %q", privKeyPath)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	privKeyPemBlock := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	if err := os.WriteFile(privKeyPath, pem.EncodeToMemory(&privKeyPemBlock), 0400); err != nil {
		return "", "", fmt.Errorf("error writing key file %q", privKeyPath)
	}

	pubKey, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", err
	}
	pubKeyPemBlock := pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKey,
	}
	if err := os.WriteFile(pubKeyPath, pem.EncodeToMemory(&pubKeyPemBlock), 0440); err != nil {
		os.Remove(privKeyPath)
		return "", "", fmt.Errorf("error writing public key file %q: %v", pubKeyPath, err)
	}

	return privKeyPath, pubKeyPath, nil
}

func generateTokenSecret(length int) ([]byte, error) {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return b, err
	}
	return b, nil
}

func configure() error {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Installing Axis body worn Swift service example version: " + version)
	fmt.Println("Built on " + buildTime)

	fmt.Println("Create a username >")
	scanner.Scan()
	user := scanner.Text()
	plaintext, hash := selectPassword()

	fmt.Println("Enter storage location >")
	scanner.Scan()
	storageLocation := scanner.Text()

	publicKey, publicKeyID := useContentEncryption()

	var port string
	fmt.Println("Choose port to use or leave empty to use 8080 >")
	fmt.Scanln(&port)
	if port == "" {
		port = "8080"
	}

	ips, err := getHostIPs()
	if err != nil || len(ips) == 0 {
		return errors.New("Failed to find any valid IP addresses")
	}

	storageLocation, err = filepath.Abs(storageLocation)
	if err != nil {
		return err
	}

	toggleHttps := yesNoQuestion("Do you want to use https? (Y/N)")

	if toggleHttps {
		ips = generateCerts(ips)
	}
	if len(ips) == 0 {
		return errors.New("Failed to generate config, couldn't generate certificates")
	}
	fmt.Println("Generating config for the the following IPs:")
	for _, ip := range ips {
		fmt.Println(ip)
	}

	tokenSecret, err := generateTokenSecret(16)
	if err != nil {
		return errors.New("Failed to generate token secret: " + err.Error())
	}

	toggleCapabilities := yesNoQuestion("Do you want to set FullStoreAndReadSupport? (Y/N)")

	// create storage location if it doesn't exist
	if _, err := os.Stat(storageLocation); os.IsNotExist(err) {
		err := os.MkdirAll(storageLocation, 0777)
		if err != nil {
			return err
		}
	}

	if !toggleCapabilities {
		err = writeCapabilities(filepath.Join(storageLocation, "System"), "Capabilities.json")
		if err != nil {
			return fmt.Errorf("Failed to write capability file: %v", err)
		}
	}

	settings := Settings{
		StorageLocation:         storageLocation,
		IPs:                     ips,
		Port:                    port,
		UseHttps:                toggleHttps,
		Username:                user,
		Password:                hash,
		plainPassword:           plaintext,
		publicKey:               publicKey,
		publicKeyID:             publicKeyID,
		TokenSecret:             tokenSecret,
		fullStoreAndReadSupport: toggleCapabilities,
	}

	confJson, _ := json.Marshal(settings)

	os.WriteFile(filepath.Join(exePath, configFilename), []byte(confJson), 0644)

	err = generateConnectionFile(settings)
	if err != nil {
		fmt.Println("Failed to generate a new Connection file")
		fmt.Println(err)
	}

	return nil
}

func generateConnectionFile(s Settings) error {
	scheme := "http://"
	if s.UseHttps {
		scheme = "https://"
	}
	ips := []string{}
	for _, ip := range s.IPs {
		s := scheme + ip + ":" + s.Port + "/auth/v1.0"
		ips = append(ips, s)
	}
	conf := Config{
		ConnectionFileVersion:   "1.0",
		ApplicationName:         "Axis body worn Swift service example",
		ApplicationVersion:      version,
		SiteName:                "Axis body worn Swift service example(" + s.IPs[0] + ")",
		BlobAPIUserName:         s.Username,
		AuthenticationTokenURI:  ips,
		BlobAPIKey:              s.plainPassword,
		BlobAPI:                 "Swift 1.0",
		ContainerType:           "mkv",
		PublicKey:               s.publicKey,
		WantEncryption:          s.publicKey != "",
		PublicKeyId:             s.publicKeyID,
		FullStoreAndReadSupport: s.fullStoreAndReadSupport,
	}
	if s.UseHttps {
		certs := []string{}
		for i, _ := range s.IPs {
			cert, err := os.ReadFile(filepath.Join(exePath, buildCertName(i)))
			if err != nil {
				return err
			}
			certs = append(certs, base64.StdEncoding.EncodeToString(cert))

		}
		conf.HTTPSCertificate = certs
	}
	jsonString, err := json.MarshalIndent(conf, "", "")
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(s.StorageLocation, "config.json"))
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(jsonString); err != nil {
		return err
	}

	fmt.Println("Successfully generated a new config file.")
	return nil
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
