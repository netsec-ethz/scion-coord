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
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"text/template"
	"time"
)

var (
	_, b, _, _      = runtime.Caller(0)
	currentPath     = filepath.Dir(b)
	scionCoordPath  = filepath.Dir(filepath.Dir(currentPath))
	localGenPath    = filepath.Join(scionCoordPath, "python", "local_gen.py")
	TempPath        = filepath.Join(scionCoordPath, "temp")
	scionPath       = filepath.Join(filepath.Dir(scionCoordPath), "scion")
	scionWebPath    = filepath.Join(scionPath, "sub", "web")
	pythonPath      = filepath.Join(scionPath, "python")
	vagrantPath     = filepath.Join(scionCoordPath, "vagrant")
	homePath        = os.Getenv("HOME")
	PackagePath     = filepath.Join(homePath, "scionLabConfigs")
	credentialsPath = filepath.Join(scionCoordPath, "credentials")
	CoreCertFile    = filepath.Join(credentialsPath, "ISD1-AS1-V0.crt")
	CoreSigKey      = filepath.Join(credentialsPath, "as-sig.key")
	TrcFile         = filepath.Join(credentialsPath, "ISD1-V0.trc")
)

type SCIONLabVMController struct {
	controllers.HTTPController
}

type SCIONLabVMInfo struct {
	IsNewUser  bool               // denotes whether this is a new user.
	UserEmail  string             // the email address of the user.
	ISD_ID     int                // ISD
	AS_ID      int                // asID of the SCIONLab VM
	IP         string             // the public IP address of the SCIONLab VM
	RemoteIA   string             // the SCIONLab AS the VM connects to
	RemoteIP   string             // the IP address of the SCIONLab AS it connects to
	RemotePort int                // port number of the remote SCIONLab AS being connected to
	SVM        *models.SCIONLabVM // if exists, the DB object that belongs to this VM
}

// The main handler function to generates a SCIONLab VM for the given user.
// If successful, it will return the filename of the tarball to download.
// The front-end will then initiate the downloading of the tarball.
func (s *SCIONLabVMController) GenerateSCIONLabVM(w http.ResponseWriter, r *http.Request) {
	// Parse the arguments
	scionLabVMIP, userEmail, err := s.parseURLParameters(r)
	if err != nil {
		log.Printf("Error parsing the parameters: %v", err)
		s.BadRequest(err, w, r)
		return
	}
	// Target SCIONLab IA to connect to is fixed for now
	targetIA := "1-7"
	isdID := 1
	svmInfo, err := s.getSCIONLabVMInfo(scionLabVMIP, userEmail, targetIA, isdID)
	if err != nil {
		log.Printf("Error getting SCIONLabVMInfo: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Generate topology file
	if err = s.generateTopologyFile(svmInfo); err != nil {
		log.Printf("Error generating topology file: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Generate local gen
	if err = s.generateLocalGen(svmInfo); err != nil {
		log.Printf("Error generating local config: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Package the VM
	var fileName string
	if fileName, err = s.packageSCIONLabVM(svmInfo.UserEmail); err != nil {
		log.Printf("Error packaging SCIONLabVM: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Persist the relevant data in the DB
	if err = s.updateDB(svmInfo); err != nil {
		log.Printf("Error updating DB tables: %v", err)
		s.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, fileName)
}

// Parses the necessary parameters from the URL: the user's email address and the public
// IP of the user's machine.
func (s *SCIONLabVMController) parseURLParameters(r *http.Request) (string, string, error) {
	if err := r.ParseForm(); err != nil {
		return "", "", fmt.Errorf("Error parsing the form: %v", err)
	}
	userEmail := r.Form["userEmail"][0]
	scionLabVMIP := r.Form["scionLabVMIP"][0]
	if scionLabVMIP == "undefined" {
		return "", "", fmt.Errorf("IP address cannot be empty. User: %v", userEmail)
	}
	return scionLabVMIP, userEmail, nil
}

// Populates and returns a SCIONLabVMInfo struct, which contains the necessary information
// to create the SCIONLab VM configuration.
func (s *SCIONLabVMController) getSCIONLabVMInfo(scionLabVMIP, userEmail, targetIA string,
	isdID int) (*SCIONLabVMInfo, error) {
	var newUser bool
	var asID int
	remotePort := -1
	// See if this user already has a VM
	svm, err := models.FindSCIONLabVMByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			newUser = true
			asID, _ = s.getNewSCIONLabVMASID()
		} else {
			return nil, fmt.Errorf("Error looking up SCIONLab VM from DB %v: %v", userEmail, err)
		}
	} else {
		newUser = false
		asID = svm.IA.As
		remotePort = svm.RemoteIAPort
	}
	log.Printf("AS ID given of the user %v: %v", userEmail, asID)
	scionLabServer, err := models.FindSCIONLabServer(targetIA)
	if err != nil {
		return nil, fmt.Errorf("Error while retrieving scionLabServer for %v: %v", targetIA, err)
	}
	log.Printf("scionLabServerIP = %v", scionLabServer.IP)
	remoteIP := scionLabServer.IP
	if newUser {
		remotePort = scionLabServer.LastAssignedPort + 1
		log.Printf("newPort to be assigned = %v", remotePort)
		scionLabServer.LastAssignedPort = remotePort
		if err := scionLabServer.Update(); err != nil {
			return nil, fmt.Errorf("Error Updating SCIONLabServerTable for SCIONLab AS: %v : %v",
				targetIA, err)
		}
	}
	return &SCIONLabVMInfo{
		IsNewUser:  newUser,
		UserEmail:  userEmail,
		ISD_ID:     isdID,
		AS_ID:      asID,
		RemoteIA:   targetIA,
		IP:         scionLabVMIP,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		SVM:        svm,
	}, nil
}

// Updates the relevant database tables related to SCIONLab VM creation.
func (s *SCIONLabVMController) updateDB(svmInfo *SCIONLabVMInfo) error {
	if svmInfo.IsNewUser {
		// update the AS table
		user, err := models.FindUserByEmail(svmInfo.UserEmail)
		if err != nil {
			return fmt.Errorf("Error finding the user by email %v: %v", svmInfo.UserEmail, err)
		}
		newAs := &models.As{
			Isd:     svmInfo.ISD_ID,
			As:      svmInfo.AS_ID,
			Core:    false,
			Account: user.Account,
			Created: time.Now().UTC(),
		}
		if err = newAs.Insert(); err != nil {
			return fmt.Errorf("Error inserting new AS: %v User: %v, %v", newAs.String(), user,
				err)
		}
		log.Printf("New SCIONLab VM AS successfully created. User: %v new AS: %v", user, newAs.String())
		newSvm := models.SCIONLabVM{
			UserEmail:    svmInfo.UserEmail,
			IP:           svmInfo.IP,
			IA:           newAs,
			RemoteIAPort: svmInfo.RemotePort,
			Activated:    false,
			RemoteIA:     svmInfo.RemoteIA,
		}
		if err = newSvm.Insert(); err != nil {
			return fmt.Errorf("Error inserting new SCIONLabVM. User: %v, %v", user, err)
		}
	} else {
		// Update the SCIONLabVM Table
		svmInfo.SVM.IP = svmInfo.IP
		if err := svmInfo.SVM.Update(); err != nil {
			return fmt.Errorf("Error updating SCIONLabVM Table. User: %v, %v", svmInfo.UserEmail,
				err)
		}
	}
	return nil
}

// Provides a new AS ID for the newly created SCIONLab VM AS.
func (s *SCIONLabVMController) getNewSCIONLabVMASID() (int, error) {
	ases, err := models.FindAllASes()
	if err != nil {
		return -1, err
	}
	// Base AS ID for SCIONLab starts from 1000
	asID := 1000
	for _, as := range ases {
		if as.As > asID {
			asID = as.As
		}
	}
	return asID + 1, nil
}

// Generates the path to the temporary topology file
func (svmInfo *SCIONLabVMInfo) topologyFile() string {
	return filepath.Join(TempPath, svmInfo.UserEmail+"_topology.json")
}

// Generates the topology file for the SCIONLab VM AS. It uses the template file
// simple_config_topo.tmpl under templates folder in order to populate and generate the
// JSON file.
func (s *SCIONLabVMController) generateTopologyFile(svmInfo *SCIONLabVMInfo) error {
	log.Printf("Generating topology file for SCIONLab VM")
	t, err := template.ParseFiles("templates/simple_config_topo.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing topology template config. User: %v, %v", svmInfo.UserEmail,
			err)
	}
	f, err := os.Create(svmInfo.topologyFile())
	if err != nil {
		return fmt.Errorf("Error creating topology file config. User: %v, %v", svmInfo.UserEmail,
			err)
	}
	// Topo file parameters
	config := map[string]string{
		"IP":           svmInfo.IP,
		"ISD_ID":       strconv.Itoa(svmInfo.ISD_ID),
		"AS_ID":        strconv.Itoa(svmInfo.AS_ID),
		"TARGET_ISDAS": svmInfo.RemoteIA,
		"REMOTE_ADDR":  svmInfo.RemoteIP,
		"REMOTE_PORT":  strconv.Itoa(svmInfo.RemotePort),
	}
	if err = t.Execute(f, config); err != nil {
		return fmt.Errorf("Error executing topology template file. User: %v, %v", svmInfo.UserEmail,
			err)
	}
	f.Close()
	return nil
}

// Creates the local gen folder of the SCIONLab VM AS. It calls a Python wrapper script
// located under the python directory. The script uses SCION's and SCION-WEB's library
// functions in order to generate the certificate, AS keys etc.
func (s *SCIONLabVMController) generateLocalGen(svmInfo *SCIONLabVMInfo) error {
	log.Printf("Creating gen folder for SCIONLab VM")
	asID := strconv.Itoa(svmInfo.AS_ID)
	isdID := strconv.Itoa(svmInfo.ISD_ID)
	userEmail := svmInfo.UserEmail
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isdID, asID, userEmail)
	cmd := exec.Command("python3", localGenPath,
		"--topo_file="+svmInfo.topologyFile(), "--user_id="+userEmail,
		"--joining_ia="+isdID+"-"+asID,
		"--core_sign_priv_key_file="+CoreSigKey,
		"--core_cert_file="+CoreCertFile,
		"--trc_file="+TrcFile,
		"--package_path="+PackagePath)
	os.Setenv("PYTHONPATH", pythonPath+":"+scionPath+":"+scionWebPath)
	cmd.Env = os.Environ()
	cmdOut, _ := cmd.StdoutPipe()
	cmdErr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Generate local gen command could not start. User: %v, %v",
			svmInfo.UserEmail, err)
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT generateLocalGen: %s\n", stdOutput)
	fmt.Printf("ERROUT generateLocalGen: %s\n", errOutput)
	return nil
}

// Packages the SCIONLab VM configuration as a tarball and returns the name of the
// generated file.
func (s *SCIONLabVMController) packageSCIONLabVM(userEmail string) (string, error) {
	log.Printf("Packaging SCIONLab VM")
	userPackagePath := filepath.Join(PackagePath, userEmail)
	vagrantDir, err := os.Open(vagrantPath)
	if err != nil {
		return "", fmt.Errorf("Failed to open directory. Path: %v, %v", vagrantPath, err)
	}
	objects, err := vagrantDir.Readdir(-1)
	if err != nil {
		return "", fmt.Errorf("Failed to read directory contents. Path: %v, %v", vagrantPath, err)
	}
	for _, obj := range objects {
		src := filepath.Join(vagrantPath, obj.Name())
		dst := filepath.Join(userPackagePath, obj.Name())
		if !obj.IsDir() {
			if err = CopyFile(src, dst); err != nil {
				return "", fmt.Errorf("Failed to copy files. User: %v, src: %v, dst: %v, %v",
					userEmail, src, dst, err)
			}
		}
	}
	cmd := exec.Command("tar", "zcvf", userEmail+".tar.gz", userEmail)
	cmd.Dir = PackagePath
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Failed to create SCIONLabVM tarball. User: %v, %v", userEmail, err)
	}
	return userEmail + ".tar.gz", nil
}

// Simple utility function to copy a file.
// TODO (ercanucan): consider moving this to a utility library.
func CopyFile(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destfile.Close()
	_, err = io.Copy(destfile, sourcefile)
	if err == nil {
		sourceinfo, _ := os.Stat(source)
		// TODO (jonghoonkwon): do proper error logging!
		err = os.Chmod(dest, sourceinfo.Mode())
	}
	return
}

type SCIONLabVMResp struct {
	ASID         string // ISD-AS of the VM
	RemoteIAPort int    // port number of the remote SCIONLab AS being connected to
	UserEmail    string // The email address of the user owning this SCIONLab VM AS
	VMIP         string // IP address of the SCIONLab VM
}

// API end-point to serve the list of newly created but not yet activated SCIONLab VMs.
// If successful, it returns an array of SCIONLabVMResp structs.
func (s *SCIONLabVMController) GetNewSCIONLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside GetNewSCIONLabVMASes = %v", r.URL.Query())
	scionLabAS := r.URL.Query().Get("scionLabAS")
	if len(scionLabAS) == 0 {
		s.BadRequest(fmt.Errorf("scionLabAS parameter missing."), w, r)
		return
	}
	vms, err := models.FindSCIONLabVMsByRemoteIA(scionLabAS)
	if err != nil {
		log.Printf("Error looking up SCIONLab VMs from DB. SCIONLabAS %v: %v", scionLabAS, err)
		s.Error500(err, w, r)
		return
	}
	vmsResp := []SCIONLabVMResp{}
	for _, v := range vms {
		vmResp := SCIONLabVMResp{
			ASID:         strconv.Itoa(v.IA.Isd) + "-" + strconv.Itoa(v.IA.As),
			VMIP:         v.IP,
			RemoteIAPort: v.RemoteIAPort,
			UserEmail:    v.UserEmail,
		}
		vmsResp = append(vmsResp, vmResp)
	}
	resp := map[string][]SCIONLabVMResp{
		scionLabAS: vmsResp,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling: %v", err)
		s.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

// API end-point to mark the provided SCIONLabVMs as ACTIVE
// If sucessful it returns and empty JSON response with HTTP code 200.
func (s *SCIONLabVMController) ConfirmActivatedSCIONLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ConfirmActivatedSCIONLabVMASes..")
	var ASIDs2VMIPs map[string][]string
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&ASIDs2VMIPs); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(err, w, r)
		return
	}
	for ia, vmIPs := range ASIDs2VMIPs {
		for _, ip := range vmIPs {
			// find the relevant SCIONLabVM and mark it as active.
			vm, err := models.FindSCIONLabVMByIPAndIA(ip, ia)
			if err != nil {
				log.Printf("Error finding the SCIONLabVM with IP %v: %v", ip, err)
				s.Error500(fmt.Errorf("Error finding the SCIONLabVM with IP %v: %v", ip, err), w, r)
				return
			}
			vm.Activated = true
			if err = vm.Update(); err != nil {
				log.Printf("Error updating SCIONLabVM Table. VM IP: %v, %v", ip, err)
				s.Error500(fmt.Errorf("Error updating SCIONLabVM Table. VM IP: %v, %v", ip, err),
					w, r)
			}
		}
	}
	fmt.Fprintln(w, "{}")
}

// API end-point to serve the generated SCIONLab VM configuration tarball.
func (s *SCIONLabVMController) ReturnTarball(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ReturnTarball = %v", r.URL.Query())
	fileName := r.URL.Query().Get("filename")
	if len(fileName) == 0 {
		s.BadRequest(fmt.Errorf("fileName parameter missing."), w, r)
		return
	}
	filePath := filepath.Join(PackagePath, fileName)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading the tarball. FileName: %v, %v", fileName, err)
		s.Error500(err, w, r)
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+fileName)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	http.ServeContent(w, r, "/api/as/downloads/"+fileName, time.Now(), bytes.NewReader(data))
}
