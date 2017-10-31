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
	"errors"
	"fmt"
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

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/email"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/netsec-ethz/scion/go/lib/addr"
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
	PackagePath     = config.PACKAGE_DIRECTORY
	credentialsPath = filepath.Join(scionCoordPath, "credentials")
	CoreCertFile    = filepath.Join(credentialsPath, "ISD1-AS1-V0.crt")
	CoreSigKey      = filepath.Join(credentialsPath, "as-sig.key")
	TrcFile         = filepath.Join(credentialsPath, "ISD1-V0.trc")
	EasyRSAPath     = filepath.Join(PackagePath, "easy-rsa")
	RSAKeyPath      = filepath.Join(EasyRSAPath, "keys")
	CACertPath      = filepath.Join(RSAKeyPath, "ca.crt")
)

// The states in which a user's SCIONLab VM can be in.
const (
	INACTIVE = iota // 0
	ACTIVE
	CREATE
	UPDATE
	REMOVE
)

// Acknowledgments for the performed operations by a SCIONLab AS.
const (
	CREATED = "Created"
	UPDATED = "Updated"
	REMOVED = "Removed"
)

func UserPackagePath(email string) string {
	return filepath.Join(PackagePath, email)
}

type SCIONLabVMController struct {
	controllers.HTTPController
}

type SCIONLabVMInfo struct {
	IsNewUser  bool               // denotes whether this is a new user.
	UserEmail  string             // the email address of the user.
	IsVPN      bool               // denotes whether this is a VPN setup
	VPNIP      string             // IP of the VPN server
	ISD_ID     int                // ISD
	AS_ID      int                // asID of the SCIONLab VM
	IP         string             // the public IP address of the SCIONLab VM
	RemoteIA   string             // the SCIONLab AS the VM connects to
	RemoteIP   string             // the IP address of the SCIONLab AS it connects to
	RemotePort int                // port number of the remote SCIONLab AS being connected to
	SVM        *models.SCIONLabVM // if exists, the DB object that belongs to this VM
}

// The main handler function to generates a SCIONLab VM for the given user.
// If successful, the front-end will initiate the downloading of the tarball.
func (s *SCIONLabVMController) GenerateSCIONLabVM(w http.ResponseWriter, r *http.Request) {
	// Parse the arguments
	isVPN, scionLabVMIP, userEmail, err := s.parseURLParameters(r)
	if err != nil {
		log.Printf("Error parsing the parameters: %v", err)
		s.BadRequest(err, w, r)
		return
	}
	// check if there is already a create or update in progress
	canCreateOrUpdate, err := s.canCreateOrUpdate(userEmail)
	if err != nil {
		log.Printf("Error checking pending create or update. User: %v, %v", userEmail, err)
		s.Error500(err, w, r)
		return
	}
	if !canCreateOrUpdate {
		s.BadRequest(fmt.Errorf("You have a pending operation. Please wait a few minutes."),
			w, r)
		return
	}
	// Target SCIONLab ISD and AS to connect to is determined by config file
	svmInfo, err := s.getSCIONLabVMInfo(scionLabVMIP, userEmail, config.SERVER_IA, isVPN)
	if err != nil {
		log.Printf("Error getting SCIONLabVMInfo: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Remove all existing files from UserPackagePath
	// TODO (mlegner): May want to archive somewhere?
	os.RemoveAll(UserPackagePath(svmInfo.UserEmail) + "/")
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
	// Generate VPN config if this is a VPN setup
	if svmInfo.IsVPN {
		if err = s.generateVPNConfig(svmInfo); err != nil {
			log.Printf("Error generating VPN config: %v", err)
			s.Error500(err, w, r)
			return
		}
	}
	// Package the VM
	if err = s.packageSCIONLabVM(svmInfo.UserEmail); err != nil {
		log.Printf("Error packaging SCIONLabVM: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Persist the relevant data into the DB
	if err = s.updateDB(svmInfo); err != nil {
		log.Printf("Error updating DB tables: %v", err)
		s.Error500(err, w, r)
		return
	}

	message := "Your VM will be activated within a few minutes. " +
		"You will receive an email confirmation as soon as the process is complete."
	fmt.Fprintln(w, message)
}

// Parses the necessary parameters from the URL: whether or not this is a VPN setup,
// the user's email address, and the public IP of the user's machine.
func (s *SCIONLabVMController) parseURLParameters(r *http.Request) (bool,
	string, string, error) {
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		return false, "", "", fmt.Errorf("Error getting the user session: %v", err)
	}
	userEmail := userSession.Email
	if err := r.ParseForm(); err != nil {
		return false, "", "", fmt.Errorf("Error parsing the form: %v", err)
	}
	scionLabVMIP := r.Form["scionLabVMIP"][0]
	isVPN, _ := strconv.ParseBool(r.Form["isVPN"][0])
	if !isVPN && scionLabVMIP == "undefined" {
		return false, "", "", fmt.Errorf(
			"IP address cannot be empty for non-VPN setup. User: %v", userEmail)
	}
	return isVPN, scionLabVMIP, userEmail, nil
}

// Check if the user's VM is already in the process of being created or updated.
func (s *SCIONLabVMController) canCreateOrUpdate(userEmail string) (bool, error) {
	svm, err := models.FindSCIONLabVMByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			return true, nil
		} else {
			return false, err
		}
	}
	if (svm.Status == ACTIVE) || (svm.Status == INACTIVE) {
		return true, nil
	}
	return false, nil
}

// Next unassigned VPN IP
func getNextVPNIP(lastAssignedIP string) (string, error) {
	if utility.IPCompare(lastAssignedIP, config.SERVER_VPN_END_IP) == -1 {
		return utility.IPIncrement(lastAssignedIP, 1), nil
	} else {
		return "", errors.New("No new VPN IP address available")
	}
}

// Populates and returns a SCIONLabVMInfo struct, which contains the necessary information
// to create the SCIONLab VM configuration.
func (s *SCIONLabVMController) getSCIONLabVMInfo(scionLabVMIP, userEmail, targetIA string,
	isVPN bool) (*SCIONLabVMInfo, error) {
	var newUser bool
	var asID int
	var ip, remoteIP, vpnIP string
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

	ia, err := addr.IAFromString(targetIA)
	if err != nil {
		return nil, err
	}

	scionLabServer, err := models.FindSCIONLabServer(targetIA)
	if err != nil {
		return nil, fmt.Errorf("Error while retrieving scionLabServer for %v: %v", targetIA, err)
	}

	// Different settings depending on whether it is a VPN or standard setup
	if isVPN {
		ip, err = getNextVPNIP(scionLabServer.VPNLastAssignedIP)
		if err != nil {
			return nil, err
		}
		scionLabServer.VPNLastAssignedIP = ip
		remoteIP = scionLabServer.VPNIP
		log.Printf("new VPN IP to be assigned to user %v: %v", userEmail, ip)
		vpnIP = scionLabServer.IP
	} else {
		ip = scionLabVMIP
		log.Printf("scionLabServerIP = %v", scionLabServer.IP)
		remoteIP = scionLabServer.IP
	}

	if newUser {
		remotePort = scionLabServer.LastAssignedPort + 1
		log.Printf("new port to be assigned to user %v: %v", userEmail, remotePort)
		scionLabServer.LastAssignedPort = remotePort
	}
	if newUser || isVPN {
		if err := scionLabServer.Update(); err != nil {
			return nil, fmt.Errorf("Error Updating SCIONLabServerTable for SCIONLab AS %v: %v",
				targetIA, err)
		}
	}

	return &SCIONLabVMInfo{
		IsNewUser:  newUser,
		UserEmail:  userEmail,
		IsVPN:      isVPN,
		ISD_ID:     ia.I,
		AS_ID:      asID,
		RemoteIA:   targetIA,
		IP:         ip,
		RemoteIP:   remoteIP,
		RemotePort: remotePort,
		VPNIP:      vpnIP,
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
		log.Printf("New SCIONLab VM AS successfully created. User: %v new AS: %v", user,
			newAs.String())
		newSvm := models.SCIONLabVM{
			UserEmail:    svmInfo.UserEmail,
			IP:           svmInfo.IP,
			IA:           newAs,
			IsVPN:        svmInfo.IsVPN,
			RemoteIAPort: svmInfo.RemotePort,
			Status:       CREATE,
			RemoteIA:     svmInfo.RemoteIA,
		}
		if err = newSvm.Insert(); err != nil {
			return fmt.Errorf("Error inserting new SCIONLabVM. User: %v, %v", user, err)
		}
	} else {
		// Update the SCIONLabVM Table
		svmInfo.SVM.IP = svmInfo.IP
		svmInfo.SVM.IsVPN = svmInfo.IsVPN
		if svmInfo.SVM.Status == INACTIVE {
			svmInfo.SVM.Status = CREATE
		} else {
			svmInfo.SVM.Status = UPDATE
		}
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
		return fmt.Errorf("Error parsing topology template config. User: %v, %v",
			svmInfo.UserEmail, err)
	}
	f, err := os.Create(svmInfo.topologyFile())
	if err != nil {
		return fmt.Errorf("Error creating topology file config. User: %v, %v", svmInfo.UserEmail,
			err)
	}
	var bindIP string
	if svmInfo.IsVPN {
		bindIP = svmInfo.IP
	} else {
		bindIP = config.VM_LOCAL_IP
	}

	// Topo file parameters
	data := map[string]string{
		"IP":           svmInfo.IP,
		"BIND_IP":      bindIP,
		"ISD_ID":       strconv.Itoa(svmInfo.ISD_ID),
		"AS_ID":        strconv.Itoa(svmInfo.AS_ID),
		"TARGET_ISDAS": svmInfo.RemoteIA,
		"REMOTE_ADDR":  svmInfo.RemoteIP,
		"REMOTE_PORT":  strconv.Itoa(svmInfo.RemotePort),
	}
	if err = t.Execute(f, data); err != nil {
		return fmt.Errorf("Error executing topology template file. User: %v, %v",
			svmInfo.UserEmail, err)
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
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isdID, asID,
		userEmail)
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
func (s *SCIONLabVMController) packageSCIONLabVM(userEmail string) error {
	log.Printf("Packaging SCIONLab VM")
	userPackagePath := UserPackagePath(userEmail)
	vagrantDir, err := os.Open(vagrantPath)
	if err != nil {
		return fmt.Errorf("Failed to open directory. Path: %v, %v", vagrantPath, err)
	}
	objects, err := vagrantDir.Readdir(-1)
	if err != nil {
		return fmt.Errorf("Failed to read directory contents. Path: %v, %v", vagrantPath, err)
	}
	for _, obj := range objects {
		src := filepath.Join(vagrantPath, obj.Name())
		dst := filepath.Join(userPackagePath, obj.Name())
		if !obj.IsDir() {
			if err = utility.CopyFile(src, dst); err != nil {
				return fmt.Errorf("Failed to copy files. User: %v, src: %v, dst: %v, %v",
					userEmail, src, dst, err)
			}
		}
	}
	cmd := exec.Command("tar", "zcvf", userEmail+".tar.gz", userEmail)
	cmd.Dir = PackagePath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to create SCIONLabVM tarball. User: %v, %v", userEmail, err)
	}
	return nil
}

// The struct used for API calls between scion-coord and SCIONLab ASes
type SCIONLabVM struct {
	ASID         string // ISD-AS of the VM
	IsVPN        bool
	RemoteIAPort int    // port number of the remote SCIONLab AS being connected to
	UserEmail    string // The email address of the user owning this SCIONLab VM AS
	VMIP         string // IP address of the SCIONLab VM
	RemoteBR     string // The name of the remote border router
}

// API end-point for the designated SCIONLab ASes to query actions to be done for users'
// SCIONLab VM ASes.
// An example response to this API may look like the following:
// {"1-7":
//        {"Create":[],
//         "Remove":[],
//         "Update":[{"ASID":"1-1020",
//                    "IsVPN":true,
//                    "RemoteIAPort":50053,
//                    "UserEmail":"user@example.com",
//                    "VMIP":"10.0.8.42",
//                    "RemoteBR":"br1-5-5"}]
//        }
// }
func (s *SCIONLabVMController) GetSCIONLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside GetSCIONLabVMASes = %v", r.URL.Query())
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
	vmsCreateResp := []SCIONLabVM{}
	vmsUpdateResp := []SCIONLabVM{}
	vmsRemoveResp := []SCIONLabVM{}
	for _, v := range vms {
		vmResp := SCIONLabVM{
			ASID:         strconv.Itoa(v.IA.Isd) + "-" + strconv.Itoa(v.IA.As),
			IsVPN:        v.IsVPN,
			VMIP:         v.IP,
			RemoteIAPort: v.RemoteIAPort,
			UserEmail:    v.UserEmail,
			RemoteBR:     v.RemoteBR,
		}
		switch v.Status {
		case CREATE:
			vmsCreateResp = append(vmsCreateResp, vmResp)
		case UPDATE:
			vmsUpdateResp = append(vmsUpdateResp, vmResp)
		case REMOVE:
			vmsRemoveResp = append(vmsRemoveResp, vmResp)
		}
	}
	resp := map[string]map[string][]SCIONLabVM{
		scionLabAS: {
			"Create": vmsCreateResp,
			"Update": vmsUpdateResp,
			"Remove": vmsRemoveResp,
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error during JSON Marshaling: %v", err)
		s.Error500(err, w, r)
		return
	}
	fmt.Fprintln(w, string(b))
}

// API end-point to mark the provided SCIONLabVMs as Created, Updated or Removed
// An example request to this API may look like the following:
// {"1-7":
//        {"Created":[],
//         "Removed":[],
//         "Updated":[{"ASID":"1-1020",
//                     "RemoteIAPort":50053,
//                     "VMIP":"203.0.113.0",
//                     "RemoteBR":"br1-5-5"}]
//        }
// }
// If sucessful, the API will return an empty JSON response with HTTP code 200.
func (s *SCIONLabVMController) ConfirmSCIONLabVMASes(w http.ResponseWriter, r *http.Request) {
	log.Printf("Inside ConfirmSCIONLabVMASes..")
	var ASIDs2VMs map[string]map[string][]SCIONLabVM
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&ASIDs2VMs); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(err, w, r)
		return
	}
	failedConfirmations := []string{}
	for ia, event := range ASIDs2VMs {
		for action, vms := range event {
			failedConfirmations = append(failedConfirmations, s.processConfirmedSCIONLabVMASes(
				ia, action, vms)...)
		}
	}
	if len(failedConfirmations) > 0 {
		s.Error500(fmt.Errorf("Error processing confirmations for the following VMs: %v",
			failedConfirmations), w, r)
		return
	}
	fmt.Fprintln(w, "{}")
}

// Updates the relevant DB tables based on the received confirmations from
// SCIONLab AS and send out confirmation emails.
func (s *SCIONLabVMController) processConfirmedSCIONLabVMASes(ia, action string,
	vms []SCIONLabVM) []string {
	log.Printf("action = %v, vms = %v", action, vms)
	failedConfirmations := []string{}
	for _, vm := range vms {
		// find the SCIONLabVM
		vm_db, err := models.FindSCIONLabVMByIPAndRemoteIA(vm.VMIP, ia)
		if err != nil {
			log.Printf("Error finding the SCIONLabVM with IP %v: %v",
				vm.VMIP, err)
			failedConfirmations = append(failedConfirmations, vm.VMIP)
			continue
		}
		switch action {
		case CREATED, UPDATED:
			vm_db.Status = ACTIVE
			vm_db.RemoteBR = vm.RemoteBR
		case REMOVED:
			vm_db.Status = INACTIVE
			vm_db.RemoteBR = ""
		default:
			log.Printf("Unsupported VM action for: %v. User %v", vm.VMIP, vm_db.UserEmail)
			failedConfirmations = append(failedConfirmations, vm.VMIP)
			continue
		}
		if err = vm_db.Update(); err != nil {
			log.Printf("Error updating SCIONLabVM Table. VM IP: %v, %v", vm.VMIP, err)
			failedConfirmations = append(failedConfirmations, vm.VMIP)
		} else {
			if err := sendConfirmationEmail(vm_db.UserEmail, action); err != nil {
				log.Printf("Error sending email confirmation to user %v: %v", vm_db.UserEmail, err)
			}
		}
	}
	return failedConfirmations
}

// Function which sends confirmation emails to users
func sendConfirmationEmail(userEmail, action string) error {
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		return err
	}

	var message string
	subject := "[SCIONLab] "
	switch action {
	case CREATED:
		message = "The infrastructure for your SCIONLab VM has been created. " +
			"You are now able to use the SCION network through your VM."
		subject += "VM creation request completed"
	case UPDATED:
		message = "The settings for your SCIONLab VM have been updated."
		subject += "VM update request completed"
	case REMOVED:
		message = "Your removal request has been processed. " +
			"All infrastructure for your SCIONLab VM has been removed."
		subject += "VM removal request completed"
	}

	data := struct {
		FirstName   string
		LastName    string
		HostAddress string
		Message     string
	}{user.FirstName, user.LastName, config.HTTP_HOST_ADDRESS, message}

	log.Printf("Sending confirmation email to user %v.", userEmail)
	if err := email.ConstructAndSend("vm_status.html", subject, data, "vm-update", userEmail); err != nil {
		return err
	}

	return nil
}

// API end-point to serve the generated SCIONLab VM configuration tarball.
func (s *SCIONLabVMController) ReturnTarball(w http.ResponseWriter, r *http.Request) {
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Forbidden(err, w, r)
		return
	}
	vm, err := models.FindSCIONLabVMByUserEmail(userSession.Email)
	if err != nil || vm.Status == INACTIVE || vm.Status == REMOVE {
		log.Printf("No active configuration found for user %v\n", userSession.Email)
		s.BadRequest(errors.New("No active configuration found"), w, r)
		return
	}

	fileName := userSession.Email + ".tar.gz"
	filePath := filepath.Join(PackagePath, fileName)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading the tarball. FileName: %v, %v", fileName, err)
		s.Error500(err, w, r)
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+fileName)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(data))
}

// The handler function to remove a SCIONLab VM for the given user.
// If successful, it will return a 200 status with an empty response.
func (s *SCIONLabVMController) RemoveSCIONLabVM(w http.ResponseWriter, r *http.Request) {
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Error500(err, w, r)
	}
	userEmail := userSession.Email

	// check if there is an active VM which can be removed
	canRemove, svm, err := s.canRemove(userEmail)
	if err != nil {
		log.Printf("Error checking if your VM can be removed. User: %v, %v", userEmail, err)
		s.Error500(err, w, r)
		return
	}
	if !canRemove {
		s.BadRequest(fmt.Errorf("You currently do not have an active SCIONLab VM."), w, r)
		return
	}
	svm.Status = REMOVE
	if err := svm.Update(); err != nil {
		s.Error500(fmt.Errorf("Error removing entry from SCIONLabVM Table. User: %v, %v",
			userEmail, err), w, r)
		return
	}
	log.Printf("Marked removal of SCIONLabVM of user %v.", userEmail)
	fmt.Fprintln(w, "Your VM will be removed within the next few minutes. "+
		"You will receive a confirmation email as soon as the removal is complete.")
}

// Check if the user's VM is already removed or in the process of being removed.
// Can remove a VM only if it is in the ACTIVE state.
func (s *SCIONLabVMController) canRemove(userEmail string) (bool, *models.SCIONLabVM, error) {
	svm, err := models.FindSCIONLabVMByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			return false, nil, nil
		} else {
			return false, nil, err
		}
	}
	if svm.Status == ACTIVE {
		return true, svm, nil
	}
	return false, nil, nil
}
