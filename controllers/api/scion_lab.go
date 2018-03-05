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
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/scionproto/scion/go/lib/addr"
)

var (
	_, b, _, _      = runtime.Caller(0)
	currentPath     = filepath.Dir(b)
	scionCoordPath  = filepath.Dir(filepath.Dir(currentPath))
	localGenPath    = filepath.Join(scionCoordPath, "python", "local_gen.py")
	TempPath        = filepath.Join(scionCoordPath, "temp")
	githubPath      = filepath.Dir(filepath.Dir(scionCoordPath))
	scionPath       = filepath.Join(githubPath, "scionproto", "scion")
	scionUtilPath   = filepath.Join(scionCoordPath, "sub", "util")
	pythonPath      = filepath.Join(scionPath, "python")
	vagrantPath     = filepath.Join(scionCoordPath, "vagrant")
	PackagePath     = config.PACKAGE_DIRECTORY
	BoxPackagePath  = filepath.Join(PackagePath, "SCIONBox")
	credentialsPath = filepath.Join(scionCoordPath, "credentials")
	EasyRSAPath     = filepath.Join(PackagePath, "easy-rsa")
	RSAKeyPath      = filepath.Join(EasyRSAPath, "keys")
	CACertPath      = filepath.Join(RSAKeyPath, "ca.crt")
	HeartBeatPeriod = time.Duration(config.HB_PERIOD)
	HeartBeatLimit  = time.Duration(config.HB_LIMIT)
)

func UserPackagePath(email string) string {
	return filepath.Join(PackagePath, email)
}

// TODO(mlegner): We need to find a better way to handle all the credential files.
func CredentialFile(isd int, ending string) string {
	return filepath.Join(credentialsPath, fmt.Sprintf("ISD%d.%s", isd, ending))
}

func CoreCertFile(isd int) string {
	return CredentialFile(isd, "crt")
}

func CoreSigKey(isd int) string {
	return CredentialFile(isd, "key")
}

func TrcFile(isd int) string {
	return CredentialFile(isd, "trc")
}

func UserPackageName(email string, isd, as int) string {
	return fmt.Sprintf("%v_%v-%v", email, isd, as)
}

func (asInfo *SCIONLabASInfo) UserPackageName() string {
	return UserPackageName(asInfo.LocalAS.UserEmail, asInfo.LocalAS.ISD, asInfo.LocalAS.ASID)
}

func (asInfo *SCIONLabASInfo) UserPackagePath() string {
	return filepath.Join(PackagePath, asInfo.UserPackageName())
}

type SCIONLabASController struct {
	controllers.HTTPController
}

type SCIONLabASInfo struct {
	IsNewConnection bool               // denotes whether this is a new user.
	IsVPN           bool               // denotes whether this is a VPN setup
	VPNServerIP     string             // IP of the VPN server
	VPNServerPort   uint16             // Port of the VPN server
	IP              string             // the public IP address of the SCIONLab AS
	LocalPort       uint16             // The port of the border router on the user side
	OldAP           string             // the previous SCIONLab AP to which the AS was connected
	RemoteIA        string             // the SCIONLab AP the AS connects to
	RemoteIP        string             // the IP address of the SCIONLab AP it connects to
	RemoteBRID      uint16             // ID of the border router in the SCIONLab AP
	RemotePort      uint16             // Port of the BR in the SCIONLab AP
	LocalAS         *models.SCIONLabAS // if exists, the DB object that belongs to this AS
	RemoteAS        *models.SCIONLabAS // the AP this AS connects to
}

type SCIONLabRequest struct {
	ASID      int    `json:"asID"`
	UserEmail string `json:"userEmail"`
	IsVPN     bool   `json:"isVPN"`
	IP        string `json:"ip"`
	ServerIA  string `json:"serverIA"`
	Label     string `json:"label"`
	Type      uint8  `json:"type"`
	Port      uint16 `json:"port"`
}

// This generates a new AS for the user if they do not have too many already
func (s *SCIONLabASController) GenerateNewSCIONLabAS(w http.ResponseWriter, r *http.Request) {
	_, uSess, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Forbidden(w, err, "Error getting the user session")
		return
	}
	ases, err := models.FindSCIONLabASesByUserEmail(uSess.Email)
	if err != nil {
		log.Printf("Error looking up current SCIONLabASes for %v: %v", uSess.Email, err)
		s.Error500(w, err, "Error looking up current SCIONLabASes")
		return
	}
	maxASes := config.MaxASes(uSess.IsAdmin)
	if len(ases) >= maxASes {
		s.Forbidden(w, nil, "You can currently only create %v ASes", maxASes)
		return
	}
	asID, err := s.getNewSCIONLabASID()
	if err != nil {
		log.Printf("Error generating new ASID for %v: %v", uSess.Email, err)
		s.Error500(w, err, "Error generating new ASID")
		return
	}
	newAS := models.SCIONLabAS{
		UserEmail: uSess.Email,
		StartPort: config.BR_START_PORT,
		ASID:      asID,
		Type:      models.VM,
		Credits:   config.VIRTUAL_CREDIT_START_CREDITS,
	}
	if err := newAS.Insert(); err != nil {
		log.Printf("Error inserting new AS for %v: %v", uSess.Email, err)
		s.Error500(w, err, "Error inserting new AS into database")
		return
	}
	fmt.Fprintf(w, "A new AS with ID %v has been generated for you. "+
		"Please use the form below to configure it.", asID)
	return
}

// The main handler function to generates a SCIONLab AS for the given user.
// If successful, the front-end will initiate the downloading of the tarball.
func (s *SCIONLabASController) ConfigureSCIONLabAS(w http.ResponseWriter, r *http.Request) {
	// Parse the arguments
	slReq, err := s.parseRequestParameters(r)
	if err != nil {
		log.Printf("Error parsing the parameters: %v", err)
		s.BadRequest(w, err, "Error parsing the parameters")
		return
	}
	// check if there is already a create or update in progress
	if err := s.canConfigure(slReq.UserEmail, slReq.ASID); err != nil {
		log.Printf("Error checking pending create or update for user %v: %v", slReq.UserEmail, err)
		s.Error500(w, err, "Error checking pending create or update")
		return
	}
	// Target SCIONLab ISD and AS to connect to is determined by config file
	asInfo, err := s.getSCIONLabASInfo(slReq)
	if err != nil {
		log.Printf("Error getting SCIONLabASInfo: %v", err)
		s.Error500(w, err, "Error getting SCIONLabASInfo")
		return
	}
	// Remove all existing files from UserPackagePath
	// TODO(mlegner): May want to archive somewhere?
	os.RemoveAll(asInfo.UserPackagePath() + "/")
	// Generate topology file
	if err = s.generateTopologyFile(asInfo); err != nil {
		log.Printf("Error generating topology file: %v", err)
		s.Error500(w, err, "Error generating topology file")
		return
	}
	// Generate local gen
	if err = s.generateLocalGen(asInfo); err != nil {
		log.Printf("Error generating local config: %v", err)
		s.Error500(w, err, "Error generating local config")
		return
	}
	// Generate VPN config if this is a VPN setup
	if asInfo.IsVPN {
		if err = s.generateVPNConfig(asInfo); err != nil {
			log.Printf("Error generating VPN config: %v", err)
			s.Error500(w, err, "Error generating VPN config")
			return
		}
	}
	// Package the SCIONLab AS configuration
	if err = s.packageConfiguration(asInfo); err != nil {
		log.Printf("Error packaging SCIONLabAS configuration: %v", err)
		s.Error500(w, err, "Error packaging SCIONLabAS configuration")
		return
	}
	// Add account id and secret to gen directory
	if err = s.createUserLoginConfiguration(asInfo); err != nil {
		log.Printf("Error generating user credential files: %v", err)
		s.Error500(w, err, "Error generating user credential files")
		return
	}
	// Persist the relevant data into the DB
	if err = s.updateDB(asInfo); err != nil {
		log.Printf("Error updating DB tables: %v", err)
		s.Error500(w, err, "Error updating DB tables")
		return
	}

	message := "Your SCIONLab AS will be activated within a few minutes. " +
		"You will receive an email confirmation as soon as the process is complete."
	fmt.Fprintln(w, message)
}

// Parses the JSON payload of the request and checks if it is valid
func (s *SCIONLabASController) parseRequestParameters(r *http.Request) (
	slReq SCIONLabRequest, err error) {
	// Get user session
	_, uSess, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		return
	}
	// parse the form value
	if err = r.ParseForm(); err != nil {
		return
	}
	// parse the JSON coming from the client
	decoder := json.NewDecoder(r.Body)
	// check if the parsing succeeded
	if err = decoder.Decode(&slReq); err != nil {
		return
	}

	// set the email address
	slReq.UserEmail = uSess.Email
	// check that ServerIA is not empty
	if slReq.ServerIA == "" {
		err = errors.New("Server IA cannot be empty.")
		return
	}
	// check that valid type is given
	if slReq.Type != models.VM && slReq.Type != models.DEDICATED {
		err = errors.New("Invalid AS type given.")
		return
	}
	// check that IP address is not empty for nonVPN setup
	if !slReq.IsVPN && slReq.IP == "" {
		err = fmt.Errorf("IP address cannot be empty for non-VPN setup. User: %v", slReq.UserEmail)
		return
	}
	return
}

// Check if the user's AS is already in the process of being created or updated.
func (s *SCIONLabASController) canConfigure(userEmail string, asID int) error {
	as, err := models.FindSCIONLabASByUserEmailAndASID(userEmail, asID)
	if err != nil {
		return err
	}
	if (as.Status == models.ACTIVE) || (as.Status == models.INACTIVE) {
		if as.Type == models.INFRASTRUCTURE {
			return errors.New("Cannot modify infrastructure ASes")
		}
		return nil
	}
	return errors.New("The given AS has a pending update request")
}

// Checks that no other AS exists with same IP address
// TODO(mlegner): This condition is more strict than necessary and should be loosened
func (s *SCIONLabASController) checkRequest(slReq SCIONLabRequest) error {
	if slReq.IsVPN {
		return nil
	}
	ases, err := models.FindSCIONLabASesByIP(slReq.IP)
	if err != nil {
		return fmt.Errorf("Error looking up ASes: %v", err)
	}
	l := len(ases)

	if l == 0 || l == 1 && ases[0].ASID == slReq.ASID {
		return nil
	}

	return fmt.Errorf("There exists another AS with the same public IP address %v", slReq.IP)
}

// Populates and returns a SCIONLabASInfo struct, which contains the necessary information
// to create the SCIONLab AS configuration.
func (s *SCIONLabASController) getSCIONLabASInfo(slReq SCIONLabRequest) (*SCIONLabASInfo, error) {
	newConnection := true
	var brID, vpnPort uint16
	var ip, remoteIP, vpnIP, oldAP string
	var cn models.ConnectionInfo
	// See if this user already has an AS
	as, err := models.FindSCIONLabASByUserEmailAndASID(slReq.UserEmail, slReq.ASID)
	if err != nil {
		return nil, fmt.Errorf("Error looking up SCIONLab AS for user %v: %v",
			slReq.UserEmail, err)
	}
	cns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return nil, fmt.Errorf("Error looking up connections of SCIONLab AS for user %v: %v",
			slReq.UserEmail, err)
	} else if len(cns) != 0 {
		oldAP = utility.IAString(cns[0].NeighborISD, cns[0].NeighborAS)
		if oldAP == slReq.ServerIA {
			newConnection = false
			brID = cn.BRID
		}
	}

	ia, err := addr.IAFromString(slReq.ServerIA)
	if err != nil {
		return nil, err
	}

	remoteAS, err := models.FindSCIONLabASByIAString(slReq.ServerIA)
	if err != nil {
		return nil, fmt.Errorf("Error while retrieving AttachmentPoint %v: %v", slReq.ServerIA, err)
	}

	// Different settings depending on whether it is a VPN or standard setup
	if slReq.IsVPN {
		if !remoteAS.AP.HasVPN {
			return nil, errors.New("The Attachment Point does not have an openVPN server running")
		}
		if !newConnection && cn.IsVPN {
			ip = cn.LocalIP
		} else {
			ip, err = remoteAS.GetFreeVPNIP()
			if err != nil {
				return nil, err
			}
			log.Printf("New VPN IP to be assigned to user %v: %v", slReq.UserEmail, ip)
		}
		remoteIP = remoteAS.AP.VPNIP
		vpnIP = remoteAS.PublicIP
		vpnPort = remoteAS.AP.VPNPort
	} else {
		ip = slReq.IP
		remoteIP = remoteAS.PublicIP
		log.Printf("IP address of AttachementPoint = %v", remoteIP)
	}

	if int(brID) < config.RESERVED_BRS_INFRASTRUCTURE {
		brID, err = remoteAS.GetFreeBRID()
		if err != nil {
			return nil, err
		}
		log.Printf("New BR ID to be assigned to user %v: %v", slReq.UserEmail, brID)
	}

	if slReq.Port > 0 {
		as.StartPort = slReq.Port
	}
	as.Type = slReq.Type
	if as.Status == models.INACTIVE {
		as.Status = models.CREATE
	} else {
		as.Status = models.UPDATE
	}
	as.PublicIP = slReq.IP
	as.ISD = ia.I
	as.Label = slReq.Label

	return &SCIONLabASInfo{
		IsNewConnection: newConnection,
		IsVPN:           slReq.IsVPN,
		RemoteIA:        slReq.ServerIA,
		IP:              ip,
		LocalPort:       as.StartPort,
		OldAP:           oldAP,
		RemoteIP:        remoteIP,
		RemoteBRID:      brID,
		RemotePort:      remoteAS.GetPortNumberFromBRID(brID),
		VPNServerIP:     vpnIP,
		VPNServerPort:   vpnPort,
		LocalAS:         as,
		RemoteAS:        remoteAS,
	}, nil
}

// Updates the relevant database tables related to SCIONLab AS creation.
func (s *SCIONLabASController) updateDB(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	if asInfo.IsNewConnection {
		// update the Connections table
		newCn := models.Connection{
			JoinIP:        asInfo.IP,
			RespondIP:     asInfo.RemoteIP,
			JoinAS:        asInfo.LocalAS,
			RespondAP:     asInfo.RemoteAS.AP,
			JoinBRID:      1,
			RespondBRID:   asInfo.RemoteBRID,
			Linktype:      models.PARENT,
			IsVPN:         asInfo.IsVPN,
			JoinStatus:    models.ACTIVE,
			RespondStatus: models.CREATE,
		}
		if err := newCn.Insert(); err != nil {
			return fmt.Errorf("Error inserting new Connection for user %v: %v",
				userEmail, err)
		}
		// update the AS database table
		if err := asInfo.LocalAS.Update(); err != nil {
			newCn.Delete()
			return fmt.Errorf("Error updating SCIONLabAS database table for user %v: %v",
				userEmail, err)
		}
		// remove the previous connection if it exists
		if asInfo.OldAP != "" {
			asInfo.LocalAS.DeleteConnectionToAP(asInfo.OldAP)
			// TODO(mlegner): Do proper error handling
		}
	} else {
		// Update the Connections Table
		cn, err := asInfo.LocalAS.GetJoinConnectionInfoToAS(asInfo.RemoteIA)
		if err != nil {
			return fmt.Errorf("Error finding existing connection of user %v: %v",
				userEmail, err)
		}
		cn.BRID = 1
		cn.IsVPN = asInfo.IsVPN
		cn.NeighborStatus = asInfo.LocalAS.Status
		cn.Status = models.ACTIVE
		if err := asInfo.LocalAS.UpdateASAndConnection(&cn); err != nil {
			return fmt.Errorf("Error updating database tables for user %v: %v",
				userEmail, err)
		}
	}
	return nil
}

// Provides a new AS ID for the newly created SCIONLab AS AS.
// TODO(mlegner): Should we maybe use the lowest unused ID instead?
func (s *SCIONLabASController) getNewSCIONLabASID() (int, error) {
	ases, err := models.FindAllASInfos()
	if err != nil {
		return -1, err
	}
	// Base AS ID for SCIONLab is set in config file
	asID := config.BASE_AS_ID
	for _, as := range ases {
		if as.ASID > asID {
			asID = as.ASID
		}
	}
	return asID + 1, nil
}

// Generates the path to the temporary topology file
func (asInfo *SCIONLabASInfo) topologyFile() string {
	return filepath.Join(TempPath, asInfo.LocalAS.IA()+"_topology.json")
}

// Generates the topology file for the SCIONLab AS AS. It uses the template file
// simple_config_topo.tmpl under templates folder in order to populate and generate the
// JSON file.
func (s *SCIONLabASController) generateTopologyFile(asInfo *SCIONLabASInfo) error {
	log.Printf("Generating topology file for SCIONLab AS")
	t, err := template.ParseFiles("templates/simple_config_topo.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing topology template config for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	f, err := os.Create(asInfo.topologyFile())
	if err != nil {
		return fmt.Errorf("Error creating topology file config for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	localIP := config.LOCALHOST_IP
	if asInfo.LocalAS.Type == models.VM {
		localIP = config.VM_LOCAL_IP
	}

	// Topo file parameters
	data := map[string]string{
		"IP":           asInfo.IP,
		"BIND_IP":      asInfo.LocalAS.BindIP(asInfo.IsVPN, asInfo.IP),
		"ISD_ID":       strconv.Itoa(asInfo.LocalAS.ISD),
		"AS_ID":        strconv.Itoa(asInfo.LocalAS.ASID),
		"LOCAL_ADDR":   localIP,
		"LOCAL_PORT":   strconv.Itoa(int(asInfo.LocalPort)),
		"TARGET_ISDAS": asInfo.RemoteIA,
		"REMOTE_ADDR":  asInfo.RemoteIP,
		"REMOTE_PORT":  strconv.Itoa(int(asInfo.RemotePort)),
	}
	if err = t.Execute(f, data); err != nil {
		return fmt.Errorf("Error executing topology template file for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	f.Close()
	return nil
}

// TODO(mlegner): Add option specifying already existing keys and certificates
// Creates the local gen folder of the SCIONLab AS AS. It calls a Python wrapper script
// located under the python directory. The script uses SCION's and SCION-WEB's library
// functions in order to generate the certificate, AS keys etc.
func (s *SCIONLabASController) generateLocalGen(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating gen folder for SCIONLab AS")
	isd := asInfo.LocalAS.ISD
	asID := asInfo.LocalAS.ASID
	userEmail := asInfo.LocalAS.UserEmail
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isd, asID,
		userEmail)
	if len(config.SIGNING_ASES) < isd {
		return fmt.Errorf("Signing AS for ISD %v not configured", isd)
	}

	cmd := exec.Command("python3", localGenPath,
		"--topo_file="+asInfo.topologyFile(), "--user_id="+asInfo.UserPackageName(),
		"--joining_ia="+utility.IAString(isd, asID),
		"--core_ia="+utility.IAString(isd, config.SIGNING_ASES[isd-1]),
		"--core_sign_priv_key_file="+CoreSigKey(isd),
		"--core_cert_file="+CoreCertFile(isd),
		"--trc_file="+TrcFile(isd),
		"--package_path="+PackagePath)
	os.Setenv("PYTHONPATH", pythonPath+":"+scionPath+":"+scionUtilPath)
	cmd.Env = os.Environ()
	cmdOut, _ := cmd.StdoutPipe()
	cmdErr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Generate local gen command could not start for user %v: %v",
			userEmail, err)
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT generateLocalGen: %s\n", stdOutput)
	fmt.Printf("ERROUT generateLocalGen: %s\n", errOutput)
	return nil
}

// TODO(mlegner): Add README for DEDICATED setup
// Packages the SCIONLab AS configuration as a tarball and returns the name of the
// generated file.
func (s *SCIONLabASController) packageConfiguration(asInfo *SCIONLabASInfo) error {
	log.Printf("Packaging SCIONLab AS")
	userEmail := asInfo.LocalAS.UserEmail
	userPackageName := asInfo.UserPackageName()
	userPackagePath := asInfo.UserPackagePath()

	// Only copy all vagrant-related files if this is a VM-type AS
	if asInfo.LocalAS.Type == models.VM {
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
					return fmt.Errorf("Failed to copy files for user %v: src: %v, dst: %v, %v",
						userEmail, src, dst, err)
				}
			}
		}

		type vagrantData struct {
			PORT_FORWARDING string
		}
		data := vagrantData{}
		if !asInfo.IsVPN {
			data.PORT_FORWARDING = fmt.Sprintf("config.vm.network \"forwarded_port\", "+
				"guest: %[1]v, host: %[1]v, protocol: \"udp\"", asInfo.LocalPort)
		}
		if err := utility.FillTemplateAndSave("templates/Vagrantfile.tmpl",
			data, filepath.Join(userPackagePath, "Vagrantfile")); err != nil {
			return err
		}
	}

	cmd := exec.Command("tar", "zcvf", userPackageName+".tar.gz", userPackageName)
	cmd.Dir = PackagePath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to create SCIONLabAS tarball for user %v: %v", userEmail, err)
	}
	return nil
}

func (s *SCIONLabASController) createUserLoginConfiguration(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating user authentication files")
	userEmail := asInfo.LocalAS.UserEmail
	acc, err := models.FindAccountByUserEmail(userEmail)
	if err != nil {
		return fmt.Errorf("Failed to find account for email: %s. %v", userEmail, err)
	}

	//TODO: maybe a better place for adding these files would be in generateLocalGen function?
	userGenDir := filepath.Join(asInfo.UserPackagePath(), "gen")

	accountId := []byte(acc.AccountID)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "account_id"), accountId, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write account ID to file. %v", err)
	}

	accountSecret := []byte(acc.Secret)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "account_secret"), accountSecret, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write account secret to file. %v", err)
	}

	ia := utility.IAString(asInfo.LocalAS.ISD, asInfo.LocalAS.ASID)
	iaString := []byte(ia)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "ia"), iaString, 0644)
	if err != nil {
		return fmt.Errorf("Failed to write IA to file. %v", err)
	}

	return nil
}

// API end-point to serve the generated SCIONLab AS configuration tarball.
func (s *SCIONLabASController) ReturnTarball(w http.ResponseWriter, r *http.Request) {
	_, uSess, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Forbidden(w, err, "Error getting the user session")
		return
	}
	vars := mux.Vars(r)
	asID := vars["as_id"]
	as, err := models.FindSCIONLabASByUserEmailAndASID(uSess.Email, asID)
	if err != nil || as.Status == models.INACTIVE || as.Status == models.REMOVE {
		log.Printf("No active configuration found for user %v\n", uSess.Email)
		s.BadRequest(w, nil, "No active configuration found for user %v",
			uSess.Email)
		return
	}

	fileName := UserPackageName(uSess.Email, as.ISD, as.ASID) + ".tar.gz"
	filePath := filepath.Join(PackagePath, fileName)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading the tarball. FileName: %v, %v", fileName, err)
		s.Error500(w, err, "Error reading tarball")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+fileName)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(data))
}

// The handler function to remove a SCIONLab AS for the given user.
// If successful, it will return a 200 status with an empty response.
func (s *SCIONLabASController) RemoveSCIONLabAS(w http.ResponseWriter, r *http.Request) {
	_, uSess, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Error500(w, err, "Error getting the user session")
	}
	userEmail := uSess.Email
	vars := mux.Vars(r)
	asID := vars["as_id"]

	// check if there is an active AS which can be removed
	canRemove, as, cn, err := s.canRemove(userEmail, asID)
	if err != nil {
		log.Printf("Error checking if your AS can be removed for user %v: %v", userEmail, err)
		s.Error500(w, err, "Error checking if AS can be removed")
		return
	}
	if !canRemove {
		s.BadRequest(w, nil, "You currently do not have an active SCIONLab AS.")
		return
	}
	as.Status = models.REMOVE
	cn.NeighborStatus = models.REMOVE
	cn.Status = models.INACTIVE
	if err := as.UpdateASAndConnection(cn); err != nil {
		log.Printf("Error marking AS and Connection as removed for user %v: %v",
			userEmail, err)
		s.Error500(w, err, "Error marking AS and Connection as removed")
		return
	}
	log.Printf("Marked removal of SCIONLabAS of user %v.", userEmail)
	fmt.Fprintln(w, "Your AS will be removed within the next few minutes. "+
		"You will receive a confirmation email as soon as the removal is complete.")
}

// Check if the user's AS is already removed or in the process of being removed.
// Can remove a AS only if it is in the ACTIVE state.
func (s *SCIONLabASController) canRemove(userEmail, asID string) (bool, *models.SCIONLabAS,
	*models.ConnectionInfo, error) {
	as, err := models.FindSCIONLabASByUserEmailAndASID(userEmail, asID)
	if err != nil {
		if err == orm.ErrNoRows {
			return false, nil, nil, nil
		} else {
			return false, nil, nil, err
		}
	}
	if as.Status == models.ACTIVE {
		if as.Type == models.INFRASTRUCTURE {
			return false, nil, nil, errors.New("Cannot remove infrastructure ASes")
		}
		cns, err := as.GetJoinConnectionInfo()
		if err != nil {
			return false, nil, nil, fmt.Errorf("Error looking up connections: %v", err)
		}
		l := len(cns)
		if err != nil || l == 0 {
			return false, nil, nil, err
		}
		if l > 1 {
			return false, nil, nil, fmt.Errorf("AS %v has currently %v connections", asID, l)
		}
		return true, as, &cns[0], nil
	}
	return false, nil, nil, nil
}

// Reads the IA parameter from the URL and returns the associated SCIONLabAS if it belongs to the
// correct account and an error otherwise
func (s *SCIONLabASController) getIAParameter(r *http.Request) (as *models.SCIONLabAS, err error) {
	ia := r.URL.Query().Get("IA")
	if len(ia) == 0 {
		err = errors.New("IA parameter missing")
		return
	}
	vars := mux.Vars(r)
	accountID := vars["account_id"]
	ases, err := models.FindSCIONLabASesByAccountID(accountID)
	for _, ownedAS := range ases {
		if ownedAS == ia {
			as, err = models.FindSCIONLabASByIAString(ia)
			return
		}
	}
	err = fmt.Errorf("The AS %v does not belong to the specified account", ia)
	return
}

// API for SCIONLabASes to query which git branch they should use for updates
func (s *SCIONLabASController) QueryUpdateBranch(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for queryUpdateBranch = %v", r.URL.Query())
	as, err := s.getIAParameter(r)
	if err != nil {
		s.BadRequest(w, err, "Incorrect IA parameter")
		return
	}
	s.Plain(as.Branch, w, r)
}

// API for SCIONLabASes to report a successful update
func (s *SCIONLabASController) ConfirmUpdate(w http.ResponseWriter, r *http.Request) {
	log.Printf("API Call for confirmUpdate = %v", r.URL.Query())
	as, err := s.getIAParameter(r)
	if err != nil {
		s.BadRequest(w, err, "Incorrect IA parameter")
		return
	}
	as.Update()
}
