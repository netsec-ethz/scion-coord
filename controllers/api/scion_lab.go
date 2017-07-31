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
	_, b, _, _       = runtime.Caller(0)
	current_path     = filepath.Dir(b)
	scion_coord_path = filepath.Dir(filepath.Dir(current_path))
	local_gen_path   = filepath.Join(scion_coord_path, "python/local_gen.py")
	// TODO (jonghoonkwon): may be better to create this topo file in a temp folder
	topo_path    = filepath.Join(scion_coord_path, "templates/simple_config_topo.json")
	scion_path   = filepath.Join(filepath.Dir(scion_coord_path), "scion")
	python_path  = filepath.Join(scion_path, "python")
	vagrant_path = filepath.Join(scion_coord_path, "vagrant")
	user_path    = filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(scion_coord_path)))))
	package_path = filepath.Join(user_path, "scionLabConfigs")
)

type SCIONLabVMController struct {
	controllers.HTTPController
}

type ScionLabVM struct {
	ASID         string // ISD-AS of the VM
	VMIP         string // IP address of the ScionLab VM
	RemoteIAPort int    // port number of the remote SCIONLab AS being connected to
}

// API end-point to serve the list of new SCIONLabVMs to the requsting SCIONLab AS
func (s *SCIONLabVMController) GetNewScionLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside GetNewScionLabVMASes = %v", r.URL.Query())
	// TODO (ercanucan): proper error handling, otherwise panic!!
	scionLabAS := r.URL.Query().Get("scionLabAS")
	vms, err := models.FindScionLabVMsByRemoteIA(scionLabAS)
	if err != nil {
		log.Printf("Error looking up SCIONLab VMs from DB. ScionLabAS %v: %v", scionLabAS, err)
		http.Error(w, "{}", http.StatusInternalServerError)
		return
	}
	log.Printf("VMs = %v", vms)
	vms_resp := []ScionLabVM{}
	for _, v := range vms {
		vm_resp := ScionLabVM{
			ASID:         strconv.Itoa(v.IA.Isd) + "-" + strconv.Itoa(v.IA.As),
			VMIP:         v.IP,
			RemoteIAPort: v.RemoteIAPort,
		}
		vms_resp = append(vms_resp, vm_resp)
	}
	resp := map[string][]ScionLabVM{
		scionLabAS: vms_resp,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling: %v ", err)
		s.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

// API end-point to mark the provided SCIONLabVMs as ACTIVE
func (s *SCIONLabVMController) ConfirmActivatedScionLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ConfirmActivatedScionLabVMASes..")
	var ASIDs2VMIPs map[string][]string

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&ASIDs2VMIPs); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(err, w, r)
		return
	}

	for ia, vmIPs := range ASIDs2VMIPs {
		for _, ip := range vmIPs {
			// find the relevant SCIONLabVM and mark as activated!
			vm, err := models.FindScionLabVMByIPAndIA(ip, ia)
			if err != nil {
				log.Printf("Error finding the ScionLabVM with IP %v: %v", ip, err)
				s.Error500(fmt.Errorf("Error finding the ScionLabVM with IP %v: %v", ip, err), w, r)
				return
			}
			vm.Activated = true
			// TODO (ercanucan): Do proper error handling here
			vm.Update()
		}
	}

	fmt.Fprintln(w, "{}")
}

func (s *SCIONLabVMController) ReturnTarball(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ReturnTarball = %v", r.URL.Query())
	file_name := r.URL.Query().Get("filename")
	file_path := filepath.Join(package_path, file_name)
	data, err := ioutil.ReadFile(file_path)
	if err != nil {
		log.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+file_name)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	http.ServeContent(w, r, "/api/as/downloads/"+file_name, time.Now(), bytes.NewReader(data))
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
	SVM        *models.ScionLabVM // if exists, the DB object that belongs to this VM
}

// Generates a SCIONLab VM
func (s *SCIONLabVMController) GenerateSCIONLabVM(w http.ResponseWriter, r *http.Request) {
	// parse the arguments
	scionLabVMIP, userEmail, err := s.parseURLParameters(r)
	if err != nil {
		// TODO (ercanucan): error handling
		s.BadRequest(err, w, r)
		return
	}
	// Target ScionLab IA to connect to is fixed for now
	targetIA := "1-7"
	isdID := 1
	svmInfo, err := s.getSCIONLabVMInfo(scionLabVMIP, userEmail, targetIA, isdID)
	if err != nil {
		// TODO (ercanucan): return a better error
		s.Error500(err, w, r)
		return
	}
	// Generate topology file
	s.generateTopologyFile(svmInfo)
	// Generate local gen
	s.generateLocalGen(svmInfo)
	// Package the VM
	fileName := s.packageSCIONLabVM(svmInfo.UserEmail)
	// Persist the relevant data in the DB
	err = s.updateDB(svmInfo)
	if err != nil {
		// TODO (ercanucan): error handling
		s.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, fileName)
}

func (s *SCIONLabVMController) parseURLParameters(r *http.Request) (string, string, error) {
	if err := r.ParseForm(); err != nil {
		log.Printf("Error while parsing the form: %v", err)
		// TODO (ercanucan): return a better error
		return "", "", err
	}
	// TODO (ercanucan): proper parsing and error handling here
	log.Printf("form is %v", r.Form)
	scionLabVMIP := r.Form["scionLabVMIP"][0]
	userEmail := r.Form["userEmail"][0]
	if scionLabVMIP == "undefined" {
		log.Printf("Empty IP address field for user %v", userEmail)
		// TODO (ercanucan): return a better error
		return "", "", fmt.Errorf("%s\n", "IP address field can not be empty.")
	}
	log.Printf("scionLabVMIP = %v", scionLabVMIP)
	log.Printf("userEmail = %v", userEmail)
	return scionLabVMIP, userEmail, nil
}

func (s *SCIONLabVMController) getSCIONLabVMInfo(scionLabVMIP, userEmail, targetIA string,
	isdID int) (*SCIONLabVMInfo, error) {
	var newUser bool
	var asID int
	remotePort := -1
	// See if this user already has a VM
	svm, err := models.FindScionLabVMByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			// TODO (ercanucan): error handling
			newUser = true
			asID, _ = s.getNewSCIONLabVMASID()
		} else {
			log.Printf("Error looking up SCIONLab VM from DB %v: %v", userEmail, err)
			// TODO (ercanucan): return a better error here
			return nil, err
		}
	} else {
		newUser = false
		log.Printf("svm.IA = %v", svm)
		asID = svm.IA.As
		remotePort = svm.RemoteIAPort
	}

	log.Printf("AS ID = %v", asID)
	scionLabServer, err := models.FindScionLabServer(targetIA)
	if err != nil {
		log.Printf("Error while retrieving scionLabServer for %v: %v", targetIA, err)
		// TODO (ercanucan): return a better error here
		return nil, err
	}

	log.Printf("scionLabServerIP = %v", scionLabServer.IP)
	remoteIP := scionLabServer.IP
	if newUser {
		remotePort = scionLabServer.LastAssignedPort + 1
		log.Printf("newPort to be assigned = %v", remotePort)
		scionLabServer.LastAssignedPort = remotePort
		if err := scionLabServer.Update(); err != nil {
			log.Printf("Error Updating ScionLabServerTable for ScionLab AS: %v : %v", targetIA, err)
			// TODO (ercanucan): return a better error here
			return nil, err
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
			log.Printf("Error finding the user by email %v: %v", svmInfo.UserEmail, err)
			// TODO (ercanucan): return a better error here
			return err
		}
		new_as := &models.As{
			Isd:     svmInfo.ISD_ID,
			As:      svmInfo.AS_ID,
			Core:    false,
			Account: user.Account,
			Created: time.Now().UTC(),
		}
		if err = new_as.Insert(); err != nil {
			log.Printf("Error inserting new AS: %v User: %v Account: %v, %v",
				new_as.String(), user, err)
			// TODO (ercanucan): return a better error here
			return err
		}
		log.Printf("New SCIONLab VM AS successfully created. User: %v new AS: %v", user, new_as.String())
		new_svm := models.ScionLabVM{
			UserEmail:    svmInfo.UserEmail,
			IP:           svmInfo.IP,
			IA:           new_as,
			RemoteIAPort: svmInfo.RemotePort,
			Activated:    false,
			RemoteIA:     svmInfo.RemoteIA,
		}
		// TODO (ercanucan): error handling
		new_svm.Insert()
	} else {
		// Update the SCIONLabVM Table
		// TODO (ercanucan): error handling
		svmInfo.SVM.IP = svmInfo.IP
		svmInfo.SVM.Update()
	}
	return nil
}

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

func (s *SCIONLabVMController) generateTopologyFile(svmInfo *SCIONLabVMInfo) {
	log.Printf("Generating topology file for SCIONLab VM")
	t, err := template.ParseFiles("templates/simple_config_topo.tmpl")
	if err != nil {
		log.Print(err)
		return
	}
	// TODO (ercanucan): create this file in a proper location!
	f, err := os.Create("templates/simple_config_topo.json")
	if err != nil {
		log.Println("create file: ", err)
		return
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
		log.Print("execute: ", err)
		return
	}
	f.Close()
}

func (s *SCIONLabVMController) generateLocalGen(svmInfo *SCIONLabVMInfo) {
	log.Printf("Creating gen folder for SCIONLab VM")
	asID := strconv.Itoa(svmInfo.AS_ID)
	isdID := strconv.Itoa(svmInfo.ISD_ID)
	userEmail := svmInfo.UserEmail
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isdID, asID, userEmail)
	cmd := exec.Command("python3", local_gen_path, "--topo_file="+topo_path, "--user_id="+userEmail,
		"--joining_ia="+isdID+"-"+asID)
	env := os.Environ()
	env = append(env, "PYTHONPATH="+python_path+":"+scion_path)
	cmd.Env = env
	cmdOut, _ := cmd.StdoutPipe()
	cmdErr, _ := cmd.StderrPipe()
	if startErr := cmd.Start(); startErr != nil {
		// TODO(ercanucan): better error reporting
		log.Printf("Generate local gen command could not start.")
		return
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT: %s\n", stdOutput)
	fmt.Printf("ERROUT: %s\n", errOutput)
}

func (s *SCIONLabVMController) packageSCIONLabVM(userEmail string) (file_path string) {
	log.Printf("Packaging SCIONLab VM")
	user_package_path := filepath.Join(user_path, "scionLabConfigs", userEmail)
	directory, _ := os.Open(vagrant_path)
	objects, err := directory.Readdir(-1)
	for _, obj := range objects {
		src := filepath.Join(vagrant_path, obj.Name())
		dst := filepath.Join(user_package_path, obj.Name())
		if !obj.IsDir() {
			if err = CopyFile(src, dst); err != nil {
				fmt.Println(err)
			}
		}
	}
	cmd := exec.Command("tar", "zcvf", userEmail+".tar.gz", userEmail)
	cmd.Dir = package_path
	if startErr := cmd.Start(); startErr != nil {
		log.Printf("Failed to create the SCIONLabVM file")
		return
	}
	file_path = userEmail + ".tar.gz"
	return
}

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
