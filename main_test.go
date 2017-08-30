package main

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestMain(t *testing.T) {
	suite.Run(t, new(MainSuite))
}

type cleanup struct {
	callback  func(string) error
	arguments string
}

type MainSuite struct {
	suite.Suite
	cleanups cleanup
}

func (suite *MainSuite) SetUpTest() {
	suite.cleanups.callback = nil
}

func (suite *MainSuite) TearDownTest() {
	cleanup := suite.cleanups.callback
	suite.cleanups.callback = nil
	if cleanup != nil {
		cleanup(suite.cleanups.arguments)
	}

}

func (suite *MainSuite) makeTempFile(content string) string {
	tmpfile, err := ioutil.TempFile("build", "test-")
	suite.NoError(err, "Failed to open the temporary file.")

	_, err = tmpfile.Write([]byte(content))
	suite.NoError(err, "Failed to write the temporary file.")

	err = tmpfile.Close()
	suite.NoError(err, "Failed to close the temporary file.")

	path := tmpfile.Name()

	suite.cleanups.callback = os.Remove
	suite.cleanups.arguments = path

	return path
}

// Read all data from `in` and return it as string.
func (suite *MainSuite) ReadAllText(in io.Reader) string {
	result, err := ioutil.ReadAll(in)
	suite.Require().NoError(err, "Failed while reading.")
	return string(result)
}

// Serialized input map to a JSON string.
func (suite *MainSuite) toJSON(input interface{}) string {
	result, err := json.Marshal(input)
	suite.Require().NoError(err, "Failed to serialized.")
	return string(result)
}

// Tests start here.

// Will use the default config file and not set the flag to show the version.
func (suite *MainSuite) TesParseCommandLineDefault() {
	arguments := []string{}

	options := parseCommandLine(arguments)

	suite.Equal("config.toml", options.configPath)
	suite.False(options.showVersion)
}

// Will signal that showing the version was requested.
func (suite *MainSuite) TestParseCommandLineShowVersion() {
	arguments := []string{"-version"}

	options := parseCommandLine(arguments)

	suite.Equal("config.toml", options.configPath)
	suite.True(options.showVersion)
}

// Can have a different configuration path.
func (suite *MainSuite) TestParseCommandLineConfigPath() {
	arguments := []string{"-config", "some/path.toml"}

	options := parseCommandLine(arguments)

	suite.Equal("some/path.toml", options.configPath)
	suite.False(options.showVersion)
}

// Will fail if the configuration file is not found
func (suite *MainSuite) TestPrepareServerConfigNotFound() {
	var options commandOptions
	options.configPath = "no/such/path.toml"
	_, _, err := prepareServer(&options)

	suite.EqualError(err, "open no/such/path.toml: no such file or directory")
}

// Will fail if the configuration file does not contains the required
// configuration section.
func (suite *MainSuite) TestPrepareServerConfigNoSection() {
	var options commandOptions
	configPath := suite.makeTempFile("[other_section]\nkey = 'value'\n")
	options.configPath = configPath
	_, _, err := prepareServer(&options)

	suite.EqualError(err, "chevah.vbox_ec2_md section not found in the configuration file")
}

// Will fail if the configuration has an invalid value for the VirtualBox
// address.
func (suite *MainSuite) TestPrepareServerInvalidVBoxURL() {
	var options commandOptions
	configPath := suite.makeTempFile(
		`[other_section]
key = 'value'
[chevah.vbox_ec2_md]
listen = 'ignored'
    [chevah.vbox_ec2_md.virtualbox]
    address = ''
`)
	options.configPath = configPath
	_, _, err := prepareServer(&options)

	suite.EqualError(err, "parse : empty url")
}

// Initialized the DB and will return the server rules together with the
// address configured to listed for connections
func (suite *MainSuite) TestPrepareServerValid() {
	var options commandOptions
	configPath := suite.makeTempFile(`[chevah.vbox_ec2_md]
listen = '1.2.3.4:1234'
    [chevah.vbox_ec2_md.virtualbox]
    address = 'http://localhost:1245'
`)
	options.configPath = configPath
	config, _, err := prepareServer(&options)

	suite.Require().NoError(err, "Failed to prepare the server.")
	suite.Equal("1.2.3.4:1234", config.Listen)
	suite.Equal("http://localhost:1245", config.VirtualBox.Address)
}

// Will reject request which don't have the HTTP POST method.
func (suite *MainSuite) TestConfigHandlerNoPost() {
	req := httptest.NewRequest("GET", "/config", nil)
	rr := httptest.NewRecorder()
	sut := http.HandlerFunc(configHandler)

	sut.ServeHTTP(rr, req)

	suite.Equal(http.StatusBadRequest, rr.Code)
	suite.Equal("Bad Request\n", rr.Body.String())
}

//
// Will reject request which don't have a body.
func (suite *MainSuite) TestConfigHandlerNoBody() {
	req := httptest.NewRequest("POST", "/config", nil)
	rr := httptest.NewRecorder()
	sut := http.HandlerFunc(configHandler)

	sut.ServeHTTP(rr, req)

	suite.Equal(http.StatusBadRequest, rr.Code)
	suite.Equal("Send a request body.\n", suite.ReadAllText(rr.Body))
}

//
// Will reject request which don't have a valid json.
func (suite *MainSuite) TestConfigHandlerInvalidBody() {
	req := httptest.NewRequest("POST", "/config", strings.NewReader("bad json"))
	rr := httptest.NewRecorder()
	sut := http.HandlerFunc(configHandler)

	sut.ServeHTTP(rr, req)

	suite.Equal(http.StatusBadRequest, rr.Code)
	suite.Equal("JSON Error: invalid character 'b' looking for beginning of value\n", suite.ReadAllText(rr.Body))
}

//
// Will reject request which don't have the machine ID.
func (suite *MainSuite) TestConfigHandlerMissingMachineID() {
	req := httptest.NewRequest("POST", "/config", strings.NewReader("{\"some_key\": \"some value\"}"))
	rr := httptest.NewRecorder()
	sut := http.HandlerFunc(configHandler)

	sut.ServeHTTP(rr, req)

	suite.Equal(http.StatusBadRequest, rr.Code)
	suite.Equal("Missing machine ID.\n", suite.ReadAllText(rr.Body))
}

//
// Will update the DB based on the POsTed configuration.
func (suite *MainSuite) TestConfigHandlerValid() {
	initDB()
	body := suite.toJSON(map[string]string{"id": "some-id", "hostname": "target-host"})
	req := httptest.NewRequest("POST", "/config", strings.NewReader(body))
	rr := httptest.NewRecorder()
	sut := http.HandlerFunc(configHandler)

	sut.ServeHTTP(rr, req)

	suite.Equal(http.StatusOK, rr.Code)
	suite.Equal("Successfully updated 'some-id'.\n", suite.ReadAllText(rr.Body))
}
