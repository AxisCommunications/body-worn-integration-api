package main

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/ncw/swift/v2"
	"golang.org/x/crypto/bcrypt"
)

const (
	UserNameTag   = "X-Auth-User"
	PasswordTag   = "X-Auth-Key"
	TokenTag      = "X-Auth-Token"
	StorageUrlTag = "X-Storage-Url"

	RootAuthEndpoint    = "/auth/v1.0"
	RootStorageEndpoint = "/v1.0/abc"

	ContainerMeta = "X-Container-Meta-"
	ObjectMeta    = "X-Object-Meta-"
)

type Settings struct {
	StorageLocation         string
	Port                    string
	IPs                     []string
	UseHttps                bool
	Username                string
	Password                []byte
	plainPassword           string
	publicKey               string
	publicKeyID             string
	TokenSecret             []byte
	fullStoreAndReadSupport bool
}

type Config struct {
	ConnectionFileVersion string `json:"ConnectionFileVersion"`
	SiteName              string `json:"SiteName"`
	ApplicationName       string `json:"ApplicationName"`
	ApplicationVersion    string `json:"ApplicationVersion"`
	//	ApplicationUsersAllowed    int    `json:"ApplicationUsersAllowed"`
	//	ApplicationDevicesAllowed  int    `json:"ApplicationDevicesAllowed"`
	//	NTPServer                  string `json:"NTPServer"`
	AuthenticationTokenURI []string `json:"AuthenticationTokenURI"`
	HTTPSCertificate       []string `json:"HTTPSCertificate,omitempty"`
	//	AxisMSSAPIVersion          string `json:"AxisMSSAPIVersion"`
	BlobAPI         string `json:"BlobAPI"`
	BlobAPIKey      string `json:"BlobAPIKey"`
	BlobAPIUserName string `json:"BlobAPIUserName"`
	ContainerType   string `json:"ContainerType"`
	//	VideoEncoding              string `json:"VideoEncoding"`
	WantEncryption          bool   `json:"WantEncryption"`
	PublicKey               string `json:"PublicKey"`
	PublicKeyId             string `json:PublicKeyId`
	FullStoreAndReadSupport bool   `json:"FullStoreAndReadSupport"`
	//	PublicKeyRenewBy           string `json:"PublicKeyRenewBy"`
	//	WantRecordingLocationFiles bool   `json:"WantRecordingLocationFiles"`
	//	WantDeviceLocationFiles    bool   `json:"WantDeviceLocationFiles"`
	//	WantRecordingAuditLog      bool   `json:"WantRecordingAuditLog"`
	//	WantDeviceAuditLog         bool   `json:"WantDeviceAuditLog"`
	//	WantRecordingDescription   bool   `json:"WantRecordingDescription"`
	//	WantRecordingCategory      bool   `json:"WantRecordingCategory"`
	//	WantRecordingTags bool `json:"WantRecordingTags"`
}

type Server struct {
	scheme   string
	settings *Settings
}

func NewServer(settings *Settings) *Server {
	return &Server{settings: settings}
}

func newError(StatusCode int, Text string) *swift.Error {
	return &swift.Error{
		StatusCode: StatusCode,
		Text:       Text,
	}
}

func (s *Server) authentication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	var username string
	var password string
	if len(r.Header[UserNameTag]) > 0 && len(r.Header[PasswordTag]) > 0 {
		username = r.Header[UserNameTag][0]
		password = r.Header[PasswordTag][0]
	} else {
		logger.Error("Call to auth without credentials")
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	AccessToken, err := auth(username, password, s.settings.Username, s.settings.Password, s.settings.TokenSecret)
	if err != nil {
		logger.Error(err)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	w.Header().Set(TokenTag, AccessToken)
	logger.Info(r.Host + RootStorageEndpoint)
	w.Header().Set(StorageUrlTag, fmt.Sprintf(s.scheme+"%s%s", r.Host, RootStorageEndpoint))
	logger.Info("User " + username + " was successfully authenticated")
}

func (s *Server) storageHandler(w http.ResponseWriter, r *http.Request) {
	if err := verifyToken(r.Header[TokenTag], s.settings.TokenSecret); err != nil {
		logger.Error(err)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodHead:
		s.handleGetMetadata(w, r)
	case http.MethodPut:
		s.handleCreation(w, r)
	case http.MethodPost:
		s.handlePostMetadata(w, r)
	case http.MethodGet:
		s.handleGet(w, r)
	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	}
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	target := getTarget(r)
	if target != "System/Capabilities.json" {
		logger.Error("Unauthorized attempt to access object: %s", target)
		e := swift.Forbidden
		http.Error(w, e.Text, e.StatusCode)
		return
	}
	data, err := os.ReadFile(filepath.Join(s.settings.StorageLocation, target))
	if err != nil {
		logger.Error(err)
		if os.IsNotExist(err) {
			e := swift.ObjectNotFound
			http.Error(w, e.Text, e.StatusCode)
			return
		}
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	digest := md5.New()
	digest.Write(data)
	hash := digest.Sum(nil)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Etag", fmt.Sprintf("%x", hash))
	_, err = w.Write(data)
	if err != nil {
		logger.Error(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

func (s *Server) handleGetMetadata(w http.ResponseWriter, r *http.Request) {
	target := getTarget(r)

	metaPath, container, err := s.getMetadataFilePath(target)
	var meta map[string]string
	if err == nil {
		meta, err = loadMetadata(metaPath)
		if err == nil {
			prefix := ObjectMeta
			if container {
				prefix = ContainerMeta
			}
			for k, v := range meta {
				w.Header().Set(prefix+k, url.PathEscape(v))
			}
			return
		}
	}

	logger.Error(err)
	e := swift.ObjectNotFound
	if container {
		e = swift.ContainerNotFound
	}
	http.Error(w, e.Text, e.StatusCode)
}

func (s *Server) getMetadataFilePath(carrier string) (string, bool, error) {
	str := strings.Split(carrier, "/")
	container := true
	if len(str) > 2 {
		return "", container, errors.New("subdirectories aren't supported")
	}
	name := filepath.Join(str[0], str[0])
	if len(str) > 1 {
		name = name + "." + str[1]
		container = false
	}
	name = name + ".metadata.json"
	return filepath.Join(s.settings.StorageLocation, name), container, nil
}

func (s *Server) handleCreation(w http.ResponseWriter, r *http.Request) {
	target := getTarget(r)

	created := true
	switch len(strings.Split(target, "/")) {
	case 1:

		logger.Info("Creating Container " + target)
		if _, err := os.Stat(filepath.Join(s.settings.StorageLocation, target)); os.IsNotExist(err) {
			if err := os.Mkdir(filepath.Join(s.settings.StorageLocation, target), 0777); err != nil {
				logger.Error(err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			created = true
		} else {
			created = false
		}

	case 2:
		fp, err := os.Create(filepath.Join(s.settings.StorageLocation, target))
		if err != nil {
			logger.Error(err)
			if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOSPC {
				http.Error(w, http.StatusText(http.StatusInsufficientStorage), http.StatusInsufficientStorage)
			} else {
				//The container doesn't exist.
				http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			}
			return
		}
		defer fp.Close()
		if _, err := io.Copy(fp, r.Body); err != nil {
			logger.Error(err)
			if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOSPC {
				http.Error(w, http.StatusText(http.StatusInsufficientStorage), http.StatusInsufficientStorage)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}
		created = true
		logger.Info("Created: " + target + "\n")

	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if e := s.handlePutMetadata(w, r, target); e != nil {
		http.Error(w, e.Text, e.StatusCode)
		return
	}
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
}

func (s *Server) handlePutMetadata(w http.ResponseWriter, r *http.Request, carrier string) *swift.Error {
	metaPath, container, err := s.getMetadataFilePath(carrier)
	if err != nil {
		logger.Error(err)
		return swift.ContainerNotFound
	}
	newMeta := parseMetadata(r)
	if container {
		oldMeta, err := loadMetadata(metaPath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			err2 := storeMetadata(metaPath, newMeta)
			if err2 != nil {
				logger.Error(err2)
				return swift.ContainerNotFound
			}
			return nil
		case err != nil:
			logger.Error(err)
			err = backupMetadata(metaPath)
			if err != nil {
				logger.Error("Failed to backup old meta data")
				logger.Error(err)
			}
			storeMetadata(metaPath, newMeta)
			return nil
		}
		updateMetadata(oldMeta, newMeta)
		storeMetadata(metaPath, oldMeta)
		return nil
	} else {
		if _, err := os.Stat(path.Dir(filepath.Join(s.settings.StorageLocation, carrier))); os.IsNotExist(err) {
			logger.Error(err)
			return swift.ObjectNotFound
		}
		if err := storeMetadata(metaPath, newMeta); err != nil {
			logger.Error(err)
			if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOSPC {
				return newError(507, "Insufficient Storage")
			}
			return swift.ObjectCorrupted

		}
		return nil
	}
}

// getTarget returns the relative filepath of the request's target file
func getTarget(r *http.Request) string {
	target := r.URL.Path[len(RootStorageEndpoint)+1:]

	// Windows doesn't accept ":" in filepaths, it needs to be escaped
	target = strings.ReplaceAll(target, ":", "_")
	return target
}

func (s *Server) handlePostMetadata(w http.ResponseWriter, r *http.Request) {
	logger.Info("Got MetadataUpdate.")
	target := getTarget(r)

	metafilename, container, err := s.getMetadataFilePath(target)

	if err != nil {
		logger.Error(err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return

	}
	newMeta := parseMetadata(r)
	if container {
		oldMeta, err := loadMetadata(metafilename)
		switch err {
		case nil:
			updateMetadata(oldMeta, newMeta)
			storeMetadata(metafilename, oldMeta)
		default:
			switch err.(type) {
			case *os.PathError:
				err2 := storeMetadata(metafilename, newMeta)
				if err2 != nil {
					logger.Error(err2)
					http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
					return
				}
				logger.Error(err)
			default:
				logger.Error(err)
				err = backupMetadata(metafilename)
				if err != nil {
					logger.Error("Failed to backup metadata")
					logger.Error(err)
				}
				storeMetadata(metafilename, newMeta)
			}
		}
		if newMeta["Status"] == "Complete" {
			_, err = os.Create(filepath.Join(s.settings.StorageLocation, target, "complete"))
			if err != nil {
				logger.Error("Failed to create a complete file")
				logger.Error(err)
			}

		}
		w.WriteHeader(http.StatusNoContent)
	} else {
		if _, err := os.Stat(filepath.Join(s.settings.StorageLocation, target)); os.IsNotExist(err) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			logger.Error(err)
			return
		}
		if err := storeMetadata(metafilename, newMeta); err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusAccepted)
		}
	}
}

// TODO maybe make sure that there can be multiple backupfiles
func backupMetadata(metadatapath string) error {
	fp, err := os.Create(metadatapath + ".bac")
	if err != nil {
		return err
	}
	defer fp.Close()
	f, err := os.Open(metadatapath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(fp, f); err != nil {
		return err
	}
	return nil

}

func URLDecode(s string) string {
	us, err := url.PathUnescape(s)
	if err != nil {
		logger.Warning("Strange encoding, returning as is: " + s)
		return s
	}
	return us
}

func parseMetadata(r *http.Request) map[string]string {
	metadata := map[string]string{}
	for k, v := range r.Header {
		if len(k) > len(ContainerMeta) && strings.HasPrefix(k, ContainerMeta) {
			metadata[strings.TrimPrefix(k, ContainerMeta)] = URLDecode(v[0])
		} else if len(k) > len(ObjectMeta) && strings.HasPrefix(k, ObjectMeta) {
			metadata[strings.TrimPrefix(k, ObjectMeta)] = URLDecode(v[0])
		}
	}
	return metadata
}

func loadMetadata(filename string) (map[string]string, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, err := io.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	err = json.Unmarshal([]byte(byteValue), &result)
	return result, err
}

func updateMetadata(oldMeta, newMeta map[string]string) {
	for k, v := range newMeta {
		if v != "" {
			oldMeta[k] = v
		} else {
			delete(oldMeta, k)
		}
	}
}

func storeMetadata(name string, metadata map[string]string) error {
	jsonString, err := json.MarshalIndent(metadata, "", "")
	if err != nil {
		logger.Error(err)
		return err
	} else {
		f, err := os.Create(name)
		if err != nil {
			logger.Error(err)
			return err
		}
		defer f.Close()
		if _, err := f.Write(jsonString); err != nil {
			logger.Error(err)
			return err
		}
	}
	return nil
}

func auth(inputUser, inputPwd, storedUser string, storedPass, tokenSecret []byte) (string, error) {

	if inputUser != storedUser {
		return "", errors.New("Access Denied, You don't have permission to access this server")
	}

	err := bcrypt.CompareHashAndPassword(storedPass, []byte(inputPwd))
	if err != nil {
		return "", errors.New("Access Denied, You don't have permission to access this server")
	}

	return createToken(tokenSecret)
}

// createToken generates a JWT token. It does not have to be JWT. It could be
// anything representable as a string.
func createToken(tokenSecret []byte) (string, error) {

	// Create claims
	claims := &jwt.StandardClaims{
		ExpiresAt: time.Now().Add(time.Minute * 15).Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	tokenString, err := token.SignedString(tokenSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func verifyToken(tokenString []string, tokenSecret []byte) error {

	if len(tokenString) == 0 {
		return errors.New("Cannot verify empty token")
	}

	// Parse token
	token, err := jwt.ParseWithClaims(tokenString[0], &jwt.StandardClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}

		return tokenSecret, nil
	})
	if err != nil {
		return errors.New("Failed to parse Token: " + err.Error())
	}

	// Validate token
	// For a token to be valid it has to be signed by tokenSecret and not yet
	// having reached its expiry time (as per StandardClaims.ExpiresAt).
	if _, ok := token.Claims.(*jwt.StandardClaims); ok && token.Valid {
		return nil
	}
	return errors.New("Failed to validate Token")
}

func startHTTPSServer(ip, port string, index int, handler http.Handler) {
	logger.Info("Server listens on " + ip + ":" + port + "...")
	err := http.ListenAndServeTLS(ip+":"+port,
		filepath.Join(exePath, buildCertName(index)),
		filepath.Join(exePath, buildKeyName(index)),
		handler)

	if err != nil {
		logger.Error("Failed to start server on " + ip + ":" + port + ", " + err.Error())
	}
}

func startHTTPServer(ip, port string, handler http.Handler) {
	logger.Info("Server listens on " + ip + ":" + port + "...")
	err := http.ListenAndServe(ip+":"+port, handler)

	if err != nil {
		logger.Error("Failed to start server on " + ip + ":" + port + ", " + err.Error())
	}

}

func (s *Server) Run() {
	logger.Info("Starting Axis body worn Swift service example: " + version)
	logger.Info("Built on " + buildTime)

	http.HandleFunc(RootAuthEndpoint, s.authentication)
	http.HandleFunc(RootStorageEndpoint+"/", s.storageHandler)

	handler := logRequestResponse(returnStatusFromEnv(http.DefaultServeMux))

	if s.settings.UseHttps {
		s.scheme = "https://"
		for i, ip := range s.settings.IPs {
			go startHTTPSServer(ip, s.settings.Port, i, handler)
		}
	} else {
		s.scheme = "http://"

		for _, ip := range s.settings.IPs {
			go startHTTPServer(ip, s.settings.Port, handler)
		}
	}
	// Block the main thread indefinitely
	select {}
}
