//Creates a test folder in the same location as the test code which later is deleted.
//Make sure no test folder exists in the storage location since it will
//get automatically deleted.

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/kardianos/service"
)

var (
	tokenSecret = []byte("secret")
)

//Check correct status is returned when doing a bad request
func TestBadRequest(t *testing.T) {
	initLogger(t)
	// PATCH is unsupported and should trigger bad request.
	req, err := http.NewRequest("PATCH", "/v1.0/abc/test/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Test", "testObjectData")
	rr := httptest.NewRecorder()
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	s.storageHandler(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

//Check GETing resources is forbidden
func TestGETAccessDenied(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})

	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}

	// GETing resource that is allowed does not return 403, indicating our token is valid.
	req, err := http.NewRequest("GET", "/v1.0/abc/System/Capabilities.json", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Test", "testObjectData")
	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Result().StatusCode != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}

	// GETing any other resource returns 403.
	req, err = http.NewRequest("GET", "/v1.0/abc/Container/somefile", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Test", "testObjectData")
	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Result().StatusCode != http.StatusForbidden {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}
}

//Check that it's not possible to do any put request without a token
func TestCallWithoutToken(t *testing.T) {
	initLogger(t)
	req, err := http.NewRequest("PUT", "/v1.0/abc/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	s.storageHandler(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}
}

//Check that it's not possible to authenticate without credentials
func TestAuthenticationWithoutCredentials(t *testing.T) {
	initLogger(t)
	req, err := http.NewRequest("GET", "/auth/v1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation})
	s.authentication(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}
}

//Check that it's not possible to authenticate with wrong username or wrong password
func TestAuthenticationWithWrongCredentials(t *testing.T) {
	initLogger(t)
	req, err := http.NewRequest("GET", "/auth/v1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-User", "test:tester")
	req.Header.Add("X-Auth-Key", "test")
	rr := httptest.NewRecorder()
	s := NewServer(&Settings{})
	s.authentication(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}

	req, err = http.NewRequest("GET", "/auth/v1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-User", "test")
	req.Header.Add("X-Auth-Key", "testing")
	rr = httptest.NewRecorder()
	s.authentication(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusUnauthorized)
	}

}

//Check that it's possible to authenticate with correct credentials
func TestAuthenticationWithCorrectCredentials(t *testing.T) {
	initLogger(t)
	req, err := http.NewRequest("GET", "/auth/v1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-User", "test:tester")
	req.Header.Add("X-Auth-Key", "testing")
	rr := httptest.NewRecorder()
	s := NewServer(&Settings{
		Username: "test:tester",
		// To change the test password: Add fmt.Prinln(string(hash)) to
		// main.go > selectPassword() and go through the install wizard.
		// Replace hash below with output.
		Password: []byte("$2a$10$opFgX6pZq0t0kRMoOZ4/J.Er7ekZ0pCxcfTinWnrUVThb64g.8Mle"),
	})
	s.authentication(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
}

//Check that the authentication endpoint returns bad request when
//recieving a bad request
func TestBadRequestToAuthenticationEndpoint(t *testing.T) {
	initLogger(t)
	req, err := http.NewRequest("POST", "/auth/v1.0", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-User", "test:tester")
	req.Header.Add("X-Auth-Key", "testing")
	rr := httptest.NewRecorder()
	s := NewServer(&Settings{})
	s.authentication(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusBadRequest)
	}
}

//Check that a container is made, that correct response is sent back and that the meta is correctly updated
func TestCreateContainer(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	status := createContainer(t, metadata, s)
	if status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
	}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
}

//Check that the response code is correct after making a duplicate container, check that the metadata is
//updated and not overwritten
func TestCreateDuplicateContainer(t *testing.T) {
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})

	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
	metadata = map[string]string{"Test-Update-Container": "updateTest"}

	resp1 = createContainer(t, metadata, s)
	if resp1 != http.StatusAccepted {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusAccepted, resp1)
	}
	metadata = map[string]string{"Test-Update-Container": "updateTest", "Test-Container": "test"}

	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
}

//Check that the response code is correct after making a duplicate container, check that metadata can be deleted
//by sending empty values
func TestDeleteContainerMetadataField(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test", "Test-Delete-Me": "deleteme"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})

	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
	metadata = map[string]string{"Test-Update-Container": "updateTest", "Test-Delete-Me": ""}

	resp1 = createContainer(t, metadata, s)
	if resp1 != http.StatusAccepted {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusAccepted, resp1)
	}
	metadata = map[string]string{"Test-Update-Container": "updateTest", "Test-Container": "test"}

	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
}

//Check post to corrupted meta
func TestPutToCorruptContainer(t *testing.T) {
	initLogger(t)
	//create a corrupted json file
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	d1 := []byte("corrupted json object")
	err := os.Mkdir(storageLocation+"/test", 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(storageLocation+"/test/test.metadata.json", d1, 0644)
	if err != nil {
		t.Fatal(err)
	}
	metadata := map[string]string{"Test-Container2": "meta3", "Test-Container3": "meta4"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusAccepted {
		t.Errorf("Error expected %v but got %v when posting to a container", http.StatusAccepted, resp1)
	}

	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
	jsonFile, err := os.Open(storageLocation + "/test/test.metadata.json.bac")
	if err != nil {
		t.Fatal(err)
	}
	defer jsonFile.Close()
	d2 := make([]byte, len(d1))
	_, err = jsonFile.Read(d2)
	if err != nil {
		t.Fatal(err)
	}

	if string(d2) != string(d1) {
		t.Fatalf("Expected \"%s\" but found \"%s\" when reading test.metadata.json.bac", string(d1), string(d2))
	}
}

//Test that post to objects returns correct response codes
//Test that meta data gets correctly overwritten
func TestPostToObject(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	resp1 = createObject(t, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating an object", http.StatusCreated, resp1)
	}
	metadata = map[string]string{"Test": "meta1", "Test2": "meta2", "Test3": "meta3"}
	resp1 = postToObject("test.txt", metadata, t, s)
	if resp1 != http.StatusAccepted {
		t.Errorf("Error expected %v but got %v when posting to an object", http.StatusAccepted, resp1)
	}
	matchMeta(t, storageLocation+"/test/test.test.txt.metadata.json", metadata)
	metadata = map[string]string{"Test4": "meta4", "Test5": "meta5"}
	resp1 = postToObject("test.txt", metadata, t, s)
	if resp1 != http.StatusAccepted {
		t.Errorf("Error expected %v but got %v when posting to an object", http.StatusAccepted, resp1)
	}
	//check that data is overwritten and not updated.
	matchMeta(t, storageLocation+"/test/test.test.txt.metadata.json", metadata)
}

//Check that the response code is correct
func TestPostToEmptyContainer(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"test": "meta1"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := postToContainer(metadata, t, s)
	//check response code
	if resp1 != http.StatusNotFound {
		t.Errorf("Error expected %v but got %v when posting to a container that doesn't exist", http.StatusNotFound, resp1)
	}
}

//Check that the response codes are correct
//Check that the meta data is updated
func TestPostToContainer(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test", "Test-Container2": "test2"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)

	metadata = map[string]string{"Test-Container2": "meta3", "Test-Container3": "meta4"}
	resp1 = postToContainer(metadata, t, s)
	if resp1 != http.StatusNoContent {
		t.Errorf("Error expected %v but got %v when posting to a container", http.StatusNoContent, resp1)
	}
	metadata = map[string]string{"Test-Container": "test", "Test-Container2": "meta3", "Test-Container3": "meta4"}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
}

//Check post to corrupted meta
func TestPostToCorruptContainer(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	//create a corrupted json file
	d1 := []byte("corrupted json object")
	err := os.Mkdir(storageLocation+"/test", 0777)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(storageLocation+"/test/test.metadata.json", d1, 0644)
	if err != nil {
		t.Fatal(err)
	}
	metadata := map[string]string{"Test-Container2": "meta3", "Test-Container3": "meta4"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := postToContainer(metadata, t, s)
	if resp1 != http.StatusNoContent {
		t.Errorf("Error expected %v but got %v when posting to a container", http.StatusNoContent, resp1)
	}

	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
	jsonFile, err := os.Open(storageLocation + "/test/test.metadata.json.bac")
	if err != nil {
		t.Fatal(err)
	}
	defer jsonFile.Close()
	d2 := make([]byte, len(d1))
	_, err = jsonFile.Read(d2)
	if err != nil {
		t.Fatal(err)
	}

	if string(d2) != string(d1) {
		t.Fatalf("Expected \"%s\" but found \"%s\" when reading test.metadata.json.bac", string(d1), string(d2))
	}
}

//Check that the correct response is sent when trying to put an object to a non existing container
func TestPutObjectToNonExistingContainer(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createObject(t, s)
	if resp1 != http.StatusNotFound {
		t.Errorf("Error expected %v but got %v when using put to an empty container", http.StatusNotFound, resp1)
	}
}

//Check that correct response is sent when trying to post to an object that doesn't exists
func TestPostToEmptyObject(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	matchMeta(t, storageLocation+"/test/test.metadata.json", metadata)
	metadata = map[string]string{"test": "meta1"}
	resp1 = postToObject("hello.txt", metadata, t, s)
	if resp1 != http.StatusNotFound {
		t.Errorf("Error expected %v but got %v when posting to an empty object", http.StatusNotFound, resp1)
	}
}

//Check correct resp code when creating an object
//Check that the object stored all data
//Check that the meta data of the object is correct
func TestCreateObject(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	resp1 = createObject(t, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating an object", http.StatusCreated, resp1)
	}
	dat, err := os.ReadFile(storageLocation + "/test/test.txt")
	if err != nil {
		t.Fatal(err)
	}

	requestBody, err := json.Marshal(map[string]string{
		"name": "Mr tester",
		"data": "axafkdsfksfs",
	})
	if string(requestBody) != string(dat) {
		t.Fatal("Error expected" + string(requestBody) + " but got " + string(dat) + " when creating an object")
	}
	meta := map[string]string{"Test": "testObjectData"}
	matchMeta(t, storageLocation+"/test/test.test.txt.metadata.json", meta)
}

//Check that a a container with the name userid_deviceid_date_time is created
func TestContainerName(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	userid := createUser(t, s)
	createDevice(t, s)
	recordingName := userid + "_AABBCCDD1234_20190101_090909"
	createRecording(t, recordingName, s)
	_, err := os.Stat(filepath.Join(storageLocation, recordingName))
	if err != nil {
		t.Fatal(err)
	}
}

//Check that a file called "complete" is created when status Complete is recieved
func TestCompleteFile(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	userid := createUser(t, s)
	createDevice(t, s)
	recordingName := userid + "_AABBCCDD1234_20190101_090909"
	createRecording(t, recordingName, s)

	_, err := os.Stat(filepath.Join(storageLocation, recordingName+"/complete"))
	if err == nil {
		t.Fatal("Error, didn't expect to find a complete file, the recording is still transfering...")
	}

	req, err := http.NewRequest("POST", "/v1.0/abc/"+recordingName, nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Status", "Complete")

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)

	_, err = os.Stat(filepath.Join(storageLocation, recordingName+"/complete"))
	if err != nil {
		t.Fatal(err)
	}
}

//Check that it's possible to get metadata from an object and container
func TestGetMetadata(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	metadata := map[string]string{"Test-Container": "test"}
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})
	resp1 := createContainer(t, metadata, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, resp1)
	}
	resp1 = createObject(t, s)
	if resp1 != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating an object", http.StatusCreated, resp1)
	}

	req, err := http.NewRequest("HEAD", "/v1.0/abc/test/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
	meta := rr.Result().Header.Get("X-Object-Meta-Test")
	if meta != "testObjectData" {
		t.Errorf("HEAD returned wrong meta data: got %s want %s", meta, "testObjectData")
	}

	req, err = http.NewRequest("HEAD", "/v1.0/abc/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusOK)
	}
	meta = rr.Result().Header.Get("X-Container-Meta-Test-Container")
	if meta != "test" {
		t.Errorf("HEAD returned wrong meta data: got %s want %s", meta, "test")
	}
}

//Check that we return the correct errors when trying to get metadata from a
//non-existing object or container
func TestGetNonexistingMetadata(t *testing.T) {
	initLogger(t)
	storageLocation, cleanUp := getStorageLocation(t)
	defer cleanUp()
	s := NewServer(&Settings{StorageLocation: storageLocation, TokenSecret: tokenSecret})

	req, err := http.NewRequest("HEAD", "/v1.0/abc/test/test.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}

	req, err = http.NewRequest("HEAD", "/v1.0/abc/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", rr.Code, http.StatusNotFound)
	}
}

func initLogger(t *testing.T) {
	svcConfig := &service.Config{Name: "M"}
	server := &mss{}
	s, err := service.New(server, svcConfig)
	if err != nil {
		t.Error(err)
	}

	logger, err = s.Logger(nil)
	if err != nil {
		t.Error(err)
	}
}

func createContainer(t *testing.T, meta map[string]string, s *Server) int {
	req, err := http.NewRequest("PUT", "/v1.0/abc/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)

	for k, v := range meta {
		req.Header.Add("X-Container-Meta-"+k, v)
	}

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	return rr.Code
}

func matchMeta(t *testing.T, metapath string, meta map[string]string) {
	jsonFile, err := os.Open(metapath)
	if err != nil {
		t.Fatal(err)
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)
	var result map[string]string
	json.Unmarshal(byteValue, &result)
	if !reflect.DeepEqual(meta, result) {
		bytes, err := json.Marshal(meta)
		if err != nil {
			t.Fatal("Metadata doesn't match expected values")
		}
		t.Fatalf("Metadata doesn't match expected values, expected:\n%s\nbut got:\n%s", string(bytes), string(byteValue))
	}
}

func postToObject(object string, meta map[string]string, t *testing.T, s *Server) int {
	req, err := http.NewRequest("POST", "/v1.0/abc/test/"+object, nil)
	if err != nil {
		t.Fatal(err)
	}

	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	for k, v := range meta {
		req.Header.Add("X-Object-Meta-"+k, v)
	}

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	return rr.Code
}

//Creates a User container and a user with a userid
func createUser(t *testing.T, s *Server) string {
	req, err := http.NewRequest("PUT", "/v1.0/abc/Users", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}

	userid := uuid.New().String()
	req, err = http.NewRequest("PUT", "/v1.0/abc/Users/"+userid, bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Name", "testUser")

	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}
	return userid

}

//Creates a Device container and a device with a deviceid
func createDevice(t *testing.T, s *Server) {
	req, err := http.NewRequest("PUT", "/v1.0/abc/Devices", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}

	req, err = http.NewRequest("PUT", "/v1.0/abc/Devices/AABBCCDD1234", bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Model", "w800")
	req.Header.Add("X-Object-Meta-Name", "BWC-01")

	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}
}

//Creates a recording with no data but with some metadata.
func createRecording(t *testing.T, recordingName string, s *Server) {
	req, err := http.NewRequest("PUT", "/v1.0/abc/"+recordingName, nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Bwcserialnumber", "AABBCCDD1234")
	req.Header.Add("X-Object-Meta-Status", "Transferring")

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}

	req, err = http.NewRequest("PUT", "/v1.0/abc/"+recordingName+"/20190101_090909_7CF7.mkv", bytes.NewBuffer(nil))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Containertype", "mkv")

	rr = httptest.NewRecorder()
	s.storageHandler(rr, req)
	if rr.Code != http.StatusCreated {
		t.Errorf("Error expected %v but got %v when creating a container", http.StatusCreated, rr.Code)
	}
}

func createObject(t *testing.T, s *Server) int {
	requestBody, err := json.Marshal(map[string]string{
		"name": "Mr tester",
		"data": "axafkdsfksfs",
	})
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest("PUT", "/v1.0/abc/test/test.txt", bytes.NewBuffer(requestBody))
	if err != nil {
		t.Fatal(err)
	}

	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	req.Header.Add("X-Object-Meta-Test", "testObjectData")
	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	return rr.Code

}

func postToContainer(meta map[string]string, t *testing.T, s *Server) int {
	req, err := http.NewRequest("POST", "/v1.0/abc/test", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, err := createToken(tokenSecret)
	if err != nil {
		t.Errorf("failed generating token while setting up test %v", err)
	}
	req.Header.Add("X-Auth-Token", token)
	for k, v := range meta {
		req.Header.Add("X-Container-Meta-"+k, v)
	}

	rr := httptest.NewRecorder()
	s.storageHandler(rr, req)
	return rr.Code
}

func getStorageLocation(t *testing.T) (string, func()) {
	dir, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Fatal(err)
	}
	return dir, func() {
		os.RemoveAll(dir)
	}
}
