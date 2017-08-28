//
// The virtualbox-metadata is a simple server which implements the EC2 meta data service.
// On top of that, it provides a proxy to the VirtualBox web service,
// so that when you are doing remote management you can use a single endpoint.
//
// It is designed to be used together with libcloud_virtualbox driver.
// https://github.com/chevah/libcloud-virtualbox
//
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/BurntSushi/toml"
)

const productVersion string = "0.1.0"

// A map with the configuration known by this server.
var machinesDB map[string]machineConfiguration

// Initialized the DB
func initDB() {
	machinesDB = make(map[string]machineConfiguration)
}

// Configuration for a machine as stored in the internal DB.
// This is also the JSON structure as sent via POST to update the configuration.
type machineConfiguration struct {
	ID       string
	Hostname string
}

type commandOptions struct {
	configPath  string
	showVersion bool
}

type tomlConfig struct {
	Chevah chevahConfig
}

type chevahConfig struct {
	VboxEC2MD vboxEC2MDConfig `toml:"vbox_ec2_md"`
}

type vboxEC2MDConfig struct {
	Listen     string
	VirtualBox virtualboxConfig
}

type virtualboxConfig struct {
	Address  string
	Username string
	Password string
}

//
// Parse the command line arguments and store the value in `result`.
func parseCommandLine(args []string) *commandOptions {
	var result commandOptions

	definition := flag.NewFlagSet("root", flag.ContinueOnError)

	definition.StringVar(
		&result.configPath,
		"config", "config.toml", "Path to the configuration file.")

	definition.BoolVar(
		&result.showVersion, "version", false, "Show the version.")

	definition.Parse(args)

	return &result
}

//
// Set up the server rules based on the command line options.
func prepareServer(options *commandOptions) (*vboxEC2MDConfig, *http.ServeMux, error) {
	var root tomlConfig
	md, err := toml.DecodeFile(options.configPath, &root)
	if err != nil {
		return nil, nil, err
	}
	config := root.Chevah.VboxEC2MD

	if md.IsDefined("chevah", "vbox_ec2_md") == false {
		return nil, nil, errors.New("chevah.vbox_ec2_md section not found in the configuration file")
	}

	vboxURL, err := url.ParseRequestURI(config.VirtualBox.Address)
	if err != nil {
		return nil, nil, err
	}

	mux := http.NewServeMux()

	// Set up reverse proxy for VirtualBox web service.
	rp := httputil.NewSingleHostReverseProxy(vboxURL)
	mux.Handle("/vbox", rp)

	// Set up handler for updating the configuration
	mux.HandleFunc("/config", configHandler)

	initDB()

	return &config, mux, nil
}

//
// Update the configuration based on the POST request.
func configHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest)
		return
	}

	var config machineConfiguration
	err := json.NewDecoder(r.Body).Decode(&config)

	if err == io.EOF {
		http.Error(w, "Send a request body.", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("JSON Error: %s", err.Error()), 400)
		return
	}

	if config.ID == "" {
		http.Error(w, "Missing machine ID.", http.StatusBadRequest)
		return
	}

	machinesDB[config.ID] = config
	fmt.Fprintf(w, "Successfully updated '%s'.\n", config.ID)

}

func main() {
	options := parseCommandLine(os.Args[1:])

	if options.showVersion {
		log.Printf("%s %s", os.Args[0], productVersion)
		os.Exit(0)
	}

	config, serverRules, err := prepareServer(options)

	if err != nil {
		log.Fatal(err.Error())
	}

	log.Printf("Starting %s (%s)", os.Args[0], productVersion)
	log.Printf("Listening on %s.", config.Listen)
	log.Printf("Reverse proxy /vbox to %s.", config.VirtualBox.Address)

	err = http.ListenAndServe(config.Listen, serverRules)
	if err != nil {
		panic(err)
	}
}
