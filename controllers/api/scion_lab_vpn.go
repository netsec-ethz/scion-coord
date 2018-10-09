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
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
)

func generateVPNConfig(asInfo *SCIONLabASInfo) error {
	return generateVPNConfigWithRetries(asInfo, 2)
}

// Generates client keys and configuration necessary for a VPN-based setup
func generateVPNConfigWithRetries(asInfo *SCIONLabASInfo, retriesLeft int) error {
	if retriesLeft <= 0 { // safeguard (just in case)
		return fmt.Errorf("Generating the VPN configuration: no more retries (see messages above)")
	}
	retry := func(originalError error) error {
		err := originalError
		if retriesLeft > 1 {
			err = cleanVPNKeys(asInfo)
			if err == nil {
				err = generateVPNConfigWithRetries(asInfo, retriesLeft-1)
			}
		}
		return err
	}
	log.Printf("Creating VPN config for SCIONLab AS")
	if err := generateVPNKeys(asInfo); err != nil {
		return retry(err)
	}
	userEmail := asInfo.LocalAS.UserEmail
	userASID := asInfo.LocalAS.ASID

	var caCert, clientCert, clientKey []byte
	caCert, err := ioutil.ReadFile(CACertPath)
	if err != nil {
		return retry(fmt.Errorf("error reading CA certificate file: %v", err))
	}
	vpnCertPath := vpnCertPath(userEmail, userASID)
	clientCert, err = ioutil.ReadFile(vpnCertPath)
	if err != nil {
		return retry(fmt.Errorf("error reading VPN certificate file for user %v: %v", userEmail, err))
	}
	clientCertStr := string(clientCert)
	startCert := strings.Index(clientCertStr, "-----BEGIN CERTIFICATE-----")
	if startCert < 0 {
		return retry(
			fmt.Errorf("Internal error: certificate file %s exists but wrong contents. Will try one more time",
				vpnCertPath))
	}
	clientKey, err = ioutil.ReadFile(vpnKeyPath(userEmail, userASID))
	if err != nil {
		return retry(fmt.Errorf("error reading VPN key file for user %v: %v", userEmail, err))
	}
	config := map[string]string{
		"ServerIP":   asInfo.VPNServerIP,
		"ServerPort": fmt.Sprintf("%v", asInfo.VPNServerPort),
		"CACert":     string(caCert),
		"ClientCert": clientCertStr[startCert:],
		"ClientKey":  string(clientKey),
	}
	err = utility.FillTemplateAndSave("templates/client.conf.tmpl", config,
		filepath.Join(asInfo.UserPackagePath(), "client.conf"))
	return err
}

// removes the files for the VPN keys
func cleanVPNKeys(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	userASID := asInfo.LocalAS.ASID
	p := vpnKeyPath(userEmail, userASID)
	_, err := os.Stat(p)
	if err == nil {
		err = os.Remove(p)
	} else if !os.IsNotExist(err) {
		err = fmt.Errorf("Cleaning VPN keys: error when stat on %s: %v", p, err)
	}
	if err != nil {
		msg := fmt.Sprintf("Cleaning VPN keys: could not remove file under %s: %v", p, err)
		log.Print(msg)
		return fmt.Errorf(msg)
	}
	p = vpnCertPath(userEmail, userASID)
	_, err = os.Stat(p)
	if err == nil {
		err = os.Remove(p)
	} else if !os.IsNotExist(err) {
		err = fmt.Errorf("Cleaning VPN keys: error when stat on %s: %v", p, err)
	}
	if err != nil {
		msg := fmt.Sprintf("Cleaning VPN keys: could not remove file under %s: %v", p, err)
		log.Print(msg)
		return fmt.Errorf(msg)
	}
	// find the key in the TXT DB and remove it:
	dbFile := filepath.Join(RSAKeyPath, "index.txt")
	if err = utility.RotateFiles(dbFile+".bak", 4); err != nil {
		return err
	}
	// write contents to index.txt:
	dbData, err := ioutil.ReadFile(dbFile)
	if err != nil {
		return err
	}
	id := vpnUserID(userEmail, userASID)
	newData := []byte{}
	scanner := bufio.NewScanner(bytes.NewReader(dbData))
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		if strings.Index(line, id) == -1 {
			newData = append(newData, line...)
		}
	}
	if err = os.Rename(dbFile, dbFile+".bak"); err != nil {
		return err
	}
	err = ioutil.WriteFile(dbFile, newData, 0664)
	return err
}

// Creates the keys for VPN setup
func generateVPNKeys(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	userASID := asInfo.LocalAS.ASID
	log.Printf("Getting RSA keys for %s %s", userEmail, userASID)
	_, err := os.Stat(vpnKeyPath(userEmail, userASID))
	if err == nil {
		_, err = os.Stat(vpnCertPath(userEmail, userASID))
	}
	if err == nil {
		log.Print("Previous VPN keys exist")
	} else if os.IsNotExist(err) {
		log.Print("Missing files, will generate them")
		cmd := exec.Command("/bin/bash", "-c", "source vars; ./build-key --batch "+
			vpnUserID(userEmail, userASID))
		cmd.Dir = EasyRSAPath
		cmdOut, _ := cmd.StdoutPipe()
		cmdErr, _ := cmd.StderrPipe()
		if err := cmd.Run(); err != nil {
			log.Printf("Error during generation of VPN keys for user %v: %v", userEmail, err)
			return err
		}

		// read stdout and stderr
		stdOutput, _ := ioutil.ReadAll(cmdOut)
		errOutput, _ := ioutil.ReadAll(cmdErr)
		fmt.Printf("STDOUT generateVPNKeys: %s\n", stdOutput)
		fmt.Printf("ERROUT generateVPNKeys: %s\n", errOutput)
	} else {
		log.Printf("Error checking for existence of VPN keys for user %v: %v", userEmail, err)
		return err
	}

	return nil
}

// Constructs the userID used as a common name for the VPN keys and certificates
func vpnUserID(userEmail string, asID addr.AS) string {
	ret := fmt.Sprintf("%s_%s", userEmail, asID.FileFmt())
	return ret
}

// Path for client key and certificate; fileExt can be "key" or "cert"
func vpnKeyCertPath(userEmail string, asID addr.AS, fileExt string) string {
	return filepath.Join(RSAKeyPath, vpnUserID(userEmail, asID)+"."+fileExt)
}

// Path for client key
func vpnKeyPath(userEmail string, asID addr.AS) string {
	return vpnKeyCertPath(userEmail, asID, "key")
}

// Path for client certificate
func vpnCertPath(userEmail string, asID addr.AS) string {
	return vpnKeyCertPath(userEmail, asID, "crt")
}
