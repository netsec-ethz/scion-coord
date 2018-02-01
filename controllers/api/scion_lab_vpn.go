// Copyright 2017 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/netsec-ethz/scion-coord/utility"
)

// Generates client keys and configuration necessary for a VPN-based setup
func (s *SCIONLabASController) generateVPNConfig(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating VPN config for SCIONLab AS")
	if err := s.generateVPNKeys(asInfo); err != nil {
		return err
	}
	userEmail := asInfo.LocalAS.UserEmail

	var caCert, clientCert, clientKey []byte
	caCert, err := ioutil.ReadFile(CACertPath)
	if err != nil {
		return fmt.Errorf("Error reading CA certificate file: %v", err)
	}
	clientCert, err = ioutil.ReadFile(filepath.Join(RSAKeyPath, userEmail+".crt"))
	if err != nil {
		return fmt.Errorf("Error reading VPN certificate file for user %v: %v",
			userEmail, err)
	}
	clientCertStr := string(clientCert)
	startCert := strings.Index(clientCertStr, "-----BEGIN CERTIFICATE-----")
	clientKey, err = ioutil.ReadFile(filepath.Join(RSAKeyPath, userEmail+".key"))
	if err != nil {
		return fmt.Errorf("Error reading VPN key file for user %v: %v",
			userEmail, err)
	}

	config := map[string]string{
		"ServerIP":   asInfo.VPNServerIP,
		"CACert":     string(caCert),
		"ClientCert": clientCertStr[startCert:],
		"ClientKey":  string(clientKey),
	}

	if err := utility.FillTemplateAndSave("templates/client.conf.tmpl",
		config, filepath.Join(asInfo.UserPackagePath(), "client.conf")); err != nil {
		return err
	}

	return nil
}

// Creates the keys for VPN setup
func (s *SCIONLabASController) generateVPNKeys(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	log.Printf("Generating RSA keys")
	if _, err := os.Stat(filepath.Join(RSAKeyPath, userEmail+".key")); err == nil {
		log.Printf("Previous VPN keys exist")
	} else if os.IsNotExist(err) {
		cmd := exec.Command("/bin/bash", "-c", "source vars; ./build-key --batch "+
			userEmail)
		cmd.Dir = EasyRSAPath
		cmdOut, _ := cmd.StdoutPipe()
		cmdErr, _ := cmd.StderrPipe()
		if err := cmd.Run(); err != nil {
			log.Printf("Error during generation of VPN keys for user %v: %v", userEmail,
				err)
			return err
		}

		// read stdout and stderr
		stdOutput, _ := ioutil.ReadAll(cmdOut)
		errOutput, _ := ioutil.ReadAll(cmdErr)
		fmt.Printf("STDOUT generateVPNKeys: %s\n", stdOutput)
		fmt.Printf("ERROUT generateVPNKeys: %s\n", errOutput)
	} else {
		log.Printf("Error checking for existence of VPN keys for user %v: %v", userEmail,
			err)
		return err
	}

	return nil
}
