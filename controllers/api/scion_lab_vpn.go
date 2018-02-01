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
	"text/template"
)

// Generates client keys and configuration necessary for a VPN-based setup
func (s *SCIONLabASController) generateVPNConfig(slasInfo *SCIONLabASInfo) error {
	log.Printf("Creating VPN config for SCIONLab AS")
	if err := s.generateVPNKeys(slasInfo); err != nil {
		return err
	}
	t, err := template.ParseFiles("templates/client.conf.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing VPN template config: %v", err)
	}

	var caCert, clientCert, clientKey []byte
	caCert, err = ioutil.ReadFile(CACertPath)
	if err != nil {
		return fmt.Errorf("Error reading CA certificate file: %v", err)
	}
	clientCert, err = ioutil.ReadFile(filepath.Join(RSAKeyPath, slasInfo.UserEmail+".crt"))
	if err != nil {
		return fmt.Errorf("Error reading VPN certificate file for user %v: %v",
			slasInfo.UserEmail, err)
	}
	clientCertStr := string(clientCert)
	startCert := strings.Index(clientCertStr, "-----BEGIN CERTIFICATE-----")
	clientKey, err = ioutil.ReadFile(filepath.Join(RSAKeyPath, slasInfo.UserEmail+".key"))
	if err != nil {
		return fmt.Errorf("Error reading VPN key file for user %v: %v",
			slasInfo.UserEmail, err)
	}

	config := map[string]string{
		"ServerIP":   slasInfo.VPNServerIP,
		"CACert":     string(caCert),
		"ClientCert": clientCertStr[startCert:],
		"ClientKey":  string(clientKey),
	}
	f, err := os.Create(filepath.Join(UserPackagePath(slasInfo.UserEmail), "client.conf"))
	if err != nil {
		return fmt.Errorf("Error creating VPN config file for user %v: %v", slasInfo.UserEmail, err)
	}
	if err = t.Execute(f, config); err != nil {
		return fmt.Errorf("Error executing VPN template file for user %v: %v",
			slasInfo.UserEmail, err)
	}
	return nil
}

// Creates the keys for VPN setup
func (s *SCIONLabASController) generateVPNKeys(slasInfo *SCIONLabASInfo) error {
	log.Printf("Generating RSA keys")
	if _, err := os.Stat(filepath.Join(RSAKeyPath, slasInfo.UserEmail+".key")); err == nil {
		log.Printf("Previous VPN keys exist")
	} else if os.IsNotExist(err) {
		cmd := exec.Command("/bin/bash", "-c", "source vars; ./build-key --batch "+
			slasInfo.UserEmail)
		cmd.Dir = EasyRSAPath
		cmdOut, _ := cmd.StdoutPipe()
		cmdErr, _ := cmd.StderrPipe()
		if err := cmd.Run(); err != nil {
			log.Printf("Error during generation of VPN keys for user %v: %v", slasInfo.UserEmail,
				err)
			return err
		}

		// read stdout and stderr
		stdOutput, _ := ioutil.ReadAll(cmdOut)
		errOutput, _ := ioutil.ReadAll(cmdErr)
		fmt.Printf("STDOUT generateVPNKeys: %s\n", stdOutput)
		fmt.Printf("ERROUT generateVPNKeys: %s\n", errOutput)
	} else {
		log.Printf("Error checking for existence of VPN keys for user %v: %v", slasInfo.UserEmail,
			err)
		return err
	}

	return nil
}
