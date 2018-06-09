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
	"crypto/rand"
	"encoding/base64"
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
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/gorilla/mux"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/netsec-ethz/scion/go/lib/crypto/cert"
	"github.com/scionproto/scion/go/lib/addr"
	"github.com/scionproto/scion/go/lib/crypto"
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
	auxFilesPath    = filepath.Join(scionCoordPath, "files")
	PackagePath     = config.PackageDirectory
	BoxPackagePath  = filepath.Join(PackagePath, "SCIONBox")
	credentialsPath = filepath.Join(scionCoordPath, "credentials")
	EasyRSAPath     = filepath.Join(PackagePath, "easy-rsa")
	RSAKeyPath      = filepath.Join(EasyRSAPath, "keys")
	CACertPath      = filepath.Join(RSAKeyPath, "ca.crt")
	HeartBeatPeriod = time.Duration(config.HeartbeatPeriod)
	HeartBeatLimit  = time.Duration(config.HeartbeatLimit)
)

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
		StartPort: config.BRStartPort,
		ASID:      asID,
		Type:      models.VM,
		Credits:   config.VirtualCreditStartCredits,
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

func generateGenForAS(asInfo *SCIONLabASInfo) error {
	var err error
	// Generate topology file
	if err = generateTopologyFile(asInfo); err != nil {
		return fmt.Errorf("Error generating topology file: %v", err)
	}
	// Generate local gen
	if err = generateLocalGen(asInfo); err != nil {
		return fmt.Errorf("Error generating local config: %v", err)
	}
	// Generate VPN config if this is a VPN setup
	if asInfo.IsVPN {
		if err = generateVPNConfig(asInfo); err != nil {
			return fmt.Errorf("Error generating VPN config: %v", err)
		}
	}
	if err = addAuxiliaryFiles(asInfo); err != nil {
		return fmt.Errorf("Error adding auxiliary files to the package: %v", err)
	}
	// Add account id and secret to gen directory
	err = createUserLoginConfiguration(asInfo)
	if err != nil {
		return fmt.Errorf("Error generating user credential files: %v", err)
	}
	// Package the SCIONLab AS configuration
	err = packageConfiguration(asInfo)
	if err != nil {
		return fmt.Errorf("Error packaging SCIONLabAS configuration: %v", err)
	}
	return nil
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
	os.RemoveAll(asInfo.UserPackagePath() + "/")
	// generate the gen folder:
	err = generateGenForAS(asInfo)
	if err != nil {
		log.Print(err)
		s.Error500(w, err, "Error generating the configuration")
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
		err = errors.New("server IA cannot be empty")
		return
	}
	// check that valid type is given
	if slReq.Type != models.VM && slReq.Type != models.Dedicated {
		err = errors.New("invalid AS type given")
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
	if (as.Status == models.Active) || (as.Status == models.Inactive) {
		if as.Type == models.Infrastructure {
			return errors.New("cannot modify infrastructure ASes")
		}
		return nil
	}
	return errors.New("the given AS has a pending update request")
}

// Checks that no other AS exists with same IP address
// TODO(mlegner): This condition is more strict than necessary and should be loosened
func (s *SCIONLabASController) checkRequest(slReq SCIONLabRequest) error {
	if slReq.IsVPN {
		return nil
	}
	ases, err := models.FindSCIONLabASesByIP(slReq.IP)
	if err != nil {
		return fmt.Errorf("error looking up ASes: %v", err)
	}
	l := len(ases)

	if l == 0 || l == 1 && ases[0].ASID == slReq.ASID {
		return nil
	}

	return fmt.Errorf("there exists another AS with the same public IP address %v", slReq.IP)
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
		return nil, fmt.Errorf("error looking up SCIONLab AS for user %v: %v",
			slReq.UserEmail, err)
	}
	cns, err := as.GetJoinConnectionInfo()
	if err != nil {
		return nil, fmt.Errorf("error looking up connections of SCIONLab AS for user %v: %v",
			slReq.UserEmail, err)
	}
	// look for an existing connection to the same AP:
	cns = models.OnlyCurrentConnections(cns)
	for _, cn = range cns {
		oldAP = utility.IAString(cn.NeighborISD, cn.NeighborAS)
		if oldAP == slReq.ServerIA {
			newConnection = false
			brID = cn.NeighborBRID
			break
		}
	}

	ia, err := addr.IAFromString(slReq.ServerIA)
	if err != nil {
		return nil, err
	}

	remoteAS, err := models.FindSCIONLabASByIAString(slReq.ServerIA)
	if err != nil {
		return nil, fmt.Errorf("error while retrieving AttachmentPoint %v: %v", slReq.ServerIA, err)
	}

	// Different settings depending on whether it is a VPN or standard setup
	if slReq.IsVPN {
		if !remoteAS.AP.HasVPN {
			return nil, errors.New("the AttachmentPoint does not have an openVPN server running")
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

	if int(brID) < config.ReservedBRsInfrastructure {
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
	if as.Status == models.Inactive {
		as.Status = models.Create
	} else {
		as.Status = models.Update
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

func getSCIONLabASInfoFromDB(conn *models.Connection) (*SCIONLabASInfo, error) {
	asInfo := SCIONLabASInfo{
		IsNewConnection: false,
		IsVPN: conn.IsVPN,
		RemoteIA: conn.RespondAP.AS.IA(),
		IP: conn.JoinIP,
		LocalPort: conn.JoinAS.StartPort,
		OldAP: "",
		RemoteIP: conn.RespondIP,
		RemoteBRID: conn.RespondBRID,
		RemotePort: conn.RespondAP.AS.GetPortNumberFromBRID(conn.RespondBRID),
		VPNServerIP: conn.RespondAP.VPNIP,
		VPNServerPort: conn.RespondAP.VPNPort,
		LocalAS: conn.JoinAS,
		RemoteAS: conn.RespondAP.AS,
	}
	return &asInfo, nil
}

// Updates the relevant database tables related to SCIONLab AS creation.
func (s *SCIONLabASController) updateDB(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	if asInfo.IsNewConnection {
		// flag the old connections for deletion:
		if asInfo.OldAP != "" {
			asInfo.LocalAS.FlagAllConnectionsToApToBeDeleted(asInfo.OldAP)
		}
		// update the Connections table
		newCn := models.Connection{
			JoinIP:        asInfo.IP,
			RespondIP:     asInfo.RemoteIP,
			JoinAS:        asInfo.LocalAS,
			RespondAP:     asInfo.RemoteAS.AP,
			JoinBRID:      1,
			RespondBRID:   asInfo.RemoteBRID,
			Linktype:      models.Parent,
			IsVPN:         asInfo.IsVPN,
			JoinStatus:    models.Active,
			RespondStatus: models.Create,
		}
		if err := newCn.Insert(); err != nil {
			return fmt.Errorf("error inserting new Connection for user %v: %v",
				userEmail, err)
		}
		// update the AS database table
		if err := asInfo.LocalAS.Update(); err != nil {
			newCn.Delete()
			return fmt.Errorf("error updating SCIONLabAS database table for user %v: %v",
				userEmail, err)
		}
	} else {
		// we had found an existing connection to the same AP.
		// Update the Connections Table
		cns, err := asInfo.LocalAS.GetJoinConnectionInfoToAS(asInfo.RemoteIA)
		if err != nil {
			return fmt.Errorf("error finding existing connection of user %v: %v",
				userEmail, err)
		}
		cns = models.OnlyCurrentConnections(cns)
		if len(cns) != 1 {
			// we've failed our assertion that there's only one active connection. Complain.
			return fmt.Errorf("error updating SCIONLabAS AS %v to AP %v: we expected 1 connection and found %v",
				asInfo.LocalAS.IA(), asInfo.RemoteIA, len(cns))
		}
		cn := cns[0]
		cn.BRID = 1
		cn.IsVPN = asInfo.IsVPN
		cn.LocalIP = asInfo.IP
		cn.NeighborIP = asInfo.RemoteIP
		cn.NeighborStatus = asInfo.LocalAS.Status
		cn.Status = models.Active
		if err := asInfo.LocalAS.UpdateASAndConnection(&cn); err != nil {
			return fmt.Errorf("error updating database tables for user %v: %v",
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
	asID := config.BaseASID
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
func generateTopologyFile(asInfo *SCIONLabASInfo) error {
	log.Printf("Generating topology file for SCIONLab AS")
	t, err := template.ParseFiles("templates/simple_config_topo.tmpl")
	if err != nil {
		return fmt.Errorf("error parsing topology template config for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	f, err := os.Create(asInfo.topologyFile())
	if err != nil {
		return fmt.Errorf("error creating topology file config for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	localIP := config.LocalhostIP
	if asInfo.LocalAS.Type == models.VM {
		localIP = config.VMLocalIP
	}

	// Topology file parameters
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
		return fmt.Errorf("error executing topology template file for user %v: %v",
			asInfo.LocalAS.UserEmail, err)
	}
	f.Close()
	return nil
}

// TODO(mlegner): Add option specifying already existing keys and certificates
// Creates the local gen folder of the SCIONLab AS AS. It calls a Python wrapper script
// located under the python directory. The script uses SCION's and SCION-WEB's library
// functions in order to generate the certificate, AS keys etc.
func generateLocalGen(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating gen folder for SCIONLab AS")
	isd := asInfo.LocalAS.ISD
	asID := asInfo.LocalAS.ASID
	userEmail := asInfo.LocalAS.UserEmail
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isd, asID,
		userEmail)
	signingAs, haveit := config.SigningASes[isd]
	if !haveit {
		return fmt.Errorf("signing AS for ISD %v not configured", isd)
	}

	cmd := exec.Command("python3", localGenPath,
		"--topo_file="+asInfo.topologyFile(), "--user_id="+asInfo.UserPackageName(),
		"--joining_ia="+utility.IAString(isd, asID),
		"--core_ia="+utility.IAString(isd, signingAs),
		"--core_sign_priv_key_file="+CoreSigKey(isd),
		"--core_cert_file="+CoreCertFile(isd),
		"--trc_file="+TrcFile(isd),
		"--package_path="+PackagePath)
	os.Setenv("PYTHONPATH", pythonPath+":"+scionPath+":"+scionUtilPath)
	cmd.Env = os.Environ()
	cmdOut, _ := cmd.StdoutPipe()
	cmdErr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("generate local gen command could not start for user %v: %v",
			userEmail, err)
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT generateLocalGen: %s\n", stdOutput)
	fmt.Printf("ERROUT generateLocalGen: %s\n", errOutput)
	return nil
}

func addAuxiliaryFiles(asInfo *SCIONLabASInfo) error {
	userEmail := asInfo.LocalAS.UserEmail
	userPackagePath := asInfo.UserPackagePath()
	log.Printf("Adding auxiliary files to the package %v", asInfo.UserPackageName())
	if asInfo.LocalAS.Type == models.Dedicated {
		dedicatedAuxFiles := filepath.Join(auxFilesPath, "dedicated_box")
		err := utility.CopyPath(dedicatedAuxFiles, userPackagePath)
		if err != nil {
			return fmt.Errorf("failed to copy files for user %v: src: %v, dst: %v, %v",
				userEmail, dedicatedAuxFiles, userPackagePath, err)
		}
	}
	return nil
}

// TODO(mlegner): Add README for Dedicated setup
// Packages the SCIONLab AS configuration as a tarball and returns the name of the
// generated file.
func packageConfiguration(asInfo *SCIONLabASInfo) error {
	log.Printf("Packaging SCIONLab AS")
	userEmail := asInfo.LocalAS.UserEmail
	userPackageName := asInfo.UserPackageName()
	userPackagePath := asInfo.UserPackagePath()

	// Only copy all vagrant-related files if this is a VM-type AS
	if asInfo.LocalAS.Type == models.VM {
		vagrantDir, err := os.Open(vagrantPath)
		if err != nil {
			return fmt.Errorf("failed to open directory. Path: %v, %v", vagrantPath, err)
		}
		objects, err := vagrantDir.Readdir(-1)
		if err != nil {
			return fmt.Errorf("failed to read directory contents. Path: %v, %v", vagrantPath, err)
		}
		for _, obj := range objects {
			src := filepath.Join(vagrantPath, obj.Name())
			dst := filepath.Join(userPackagePath, obj.Name())
			if !obj.IsDir() {
				if err = utility.CopyFile(src, dst); err != nil {
					return fmt.Errorf("failed to copy files for user %v: src: %v, dst: %v, %v",
						userEmail, src, dst, err)
				}
			}
		}

		data := map[string]string{}
		if !asInfo.IsVPN {
			data["PORT_FORWARDING"] = fmt.Sprintf("config.vm.network \"forwarded_port\", "+
				"guest: %[1]v, host: %[1]v, protocol: \"udp\"", asInfo.LocalPort)
		}
		if err := utility.FillTemplateAndSave("templates/Vagrantfile.tmpl",
			data, filepath.Join(userPackagePath, "Vagrantfile")); err != nil {
			return err
		}
	}

	cmd := exec.Command("tar", "zcvf", userPackageName+".tar.gz", userPackageName)
	cmd.Dir = PackagePath
	err := cmd.Start()
	if err == nil {
		err = cmd.Wait()
	}
	if err != nil {
		return fmt.Errorf("failed to create SCIONLabAS tarball for user %v: %v", userEmail, err)
	}

	return nil
}

func createUserLoginConfiguration(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating user authentication files")
	userEmail := asInfo.LocalAS.UserEmail
	acc, err := models.FindAccountByUserEmail(userEmail)
	if err != nil {
		return fmt.Errorf("failed to find account for email %s: %v", userEmail, err)
	}

	userGenDir := filepath.Join(asInfo.UserPackagePath(), "gen")

	accountId := []byte(acc.AccountID)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "account_id"), accountId, 0644)
	if err != nil {
		return fmt.Errorf("failed to write account ID to file: %v", err)
	}

	accountSecret := []byte(acc.Secret)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "account_secret"), accountSecret, 0644)
	if err != nil {
		return fmt.Errorf("failed to write account secret to file: %v", err)
	}

	ia := utility.IAString(asInfo.LocalAS.ISD, asInfo.LocalAS.ASID)
	iaString := []byte(ia)
	err = ioutil.WriteFile(filepath.Join(userGenDir, "ia"), iaString, 0644)
	if err != nil {
		return fmt.Errorf("failed to write IA to file: %v", err)
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
	if err != nil || as.Status == models.Inactive || as.Status == models.Remove {
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

// RemapASIdentityChallengeAndSolution returns the challenge the AS should solve if said AS has to map the identity.
func (s *SCIONLabASController) RemapASIdentityChallengeAndSolution(w http.ResponseWriter, r *http.Request) {
	// TODO: remove debug print s
	// TODO: refactor
	answer := make(map[string]interface{})
	answer["error"] = false

	vars := mux.Vars(r)
	asID := vars["as_id"]
	as, err := models.FindSCIONLabASByIAString(asID)
	if err != nil {
		answer["error"] = true
		answer["msg"] = fmt.Sprintf("Could not find AS with IA %v", asID)
		utility.SendJSONError(answer, w)
		return
	}
	answeringChallenge := false
	if r.Method == http.MethodPost {
		answeringChallenge = true
	}

	request := make(map[string]interface{})
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		answer["error"] = true
		answer["msg"] = fmt.Sprintf("Could not read JSON in the request: %v", err)
		utility.SendJSONError(answer, w)
		return
	}
	json.Unmarshal(body, &request)
	fmt.Println(" ########### request: ", request)
	if answeringChallenge {
		// check we have the needed fields
		_, havechallenge := request["challenge"]
		_, haveanswer := request["answer"]
		if !havechallenge || !haveanswer {
			answer["msg"] = `JSON missing "challenge" or "answer"`
			answer["error"] = true
			utility.SendJSONError(answer, w)
			return
		}
	}
	// check if the existing AS needs remapping
	if !as.AreIDsFromScionLab() {
		oldanswer := make(map[string]interface{})
		err = json.Unmarshal([]byte(as.RemapStatus), &oldanswer)
		if err != nil {
			log.Printf("There was an error recovering RemapStatus for %s: %v", asID, err)
			oldanswer = make(map[string]interface{})
		}
		randomBytes := make([]byte, 512)
		pending, pendingExists := oldanswer["pending"]
		challenge, challengeExists := oldanswer["challenge"]
		if challengeExists && pendingExists && pending.(bool) {
			randomBytes, err = base64.StdEncoding.DecodeString(challenge.(string))
			if err != nil {
				// internal logic error
				log.Printf("Logic error, failed to base64 decode stored challenge: %v", err)
				return
			}
			answer["pending"] = pending
		} else {
			answer["pending"] = true
			_, err = rand.Read(randomBytes)
			if err != nil {
				answer["error"] = true
				answer["msg"] = "Could not create challenge"
				utility.SendJSONError(answer, w)
				return
			}
		}
		answer["challenge"] = base64.StdEncoding.EncodeToString(randomBytes)
		if !pendingExists {
			// save the challenge to DB
			marshalled, _ := json.Marshal(answer)
			as.RemapStatus = string(marshalled)
			err = as.Update()
			if err != nil {
				answer["error"] = true
				answer["msg"] = fmt.Sprintf("Could not update challenge for AS: %v", err)
				utility.SendJSONError(answer, w)
				return
			}
		}
		if answeringChallenge {
			if request["challenge"] != answer["challenge"] {
				msg := fmt.Sprintf("Different challenge being solved for IA %s.\nChallenge in DB   : %v\nChallenge received: %v", asID, answer["challenge"], request["challenge"])
				answer["error"] = true
				answer["msg"] = msg
				utility.SendJSONError(answer, w)
				return
			}
			receivedSignature, err := base64.StdEncoding.DecodeString(request["answer"].(string))
			if err != nil {
				answer["error"] = true
				answer["msg"] = "Cannot decode the answer to the challenge"
				utility.SendJSONError(answer, w)
				return
			}
			path := filepath.Join(PackagePath, UserPackageName(as.UserEmail, as.ISD, as.ASID), "gen", fmt.Sprintf("ISD%d", as.ISD), fmt.Sprintf("AS%d", as.ASID),
				fmt.Sprintf("bs%d-%d-1", as.ISD, as.ASID), "certs")
			var chain *cert.Chain
			fileInfos, err := ioutil.ReadDir(path)
			if err == nil {
				possibleCerts := []string{}
				for _, f := range fileInfos {
					if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".crt") {
						possibleCerts = append(possibleCerts, f.Name())
					}
				}
				if len(possibleCerts) < 1 {
					err = fmt.Errorf("Cannot find any .crt file for IA %v", asID)
				} else {
					sort.Sort(sort.Reverse(sort.StringSlice(possibleCerts)))
					fmt.Println("DEBUG: possible certs: ", possibleCerts)
					path = filepath.Join(path, possibleCerts[0])
					// TODO: deleteme
					path = "/home/juan/go/src/github.com/scionproto/scion/gen/ISD1/AS1010/bs1-1010-1/certs/ISD1-AS1010-V1.crt"
					chainBytes, err := ioutil.ReadFile(path)
					if err == nil {
						chain, err = cert.ChainFromRaw(chainBytes, false)
					}
				}
			}
			// we could assert that chain is not null iff err == nil, but we also test it istead:
			if err != nil || chain == nil {
				// TODO: this is actually very bad, do we delete the AS entry here? what do we do?
				answer["error"] = true
				msg := fmt.Sprintf("ERROR in Coordinator: cannot load the public certificate for AS %s : %v", asID, err)
				answer["msg"] = msg
				log.Print(msg)
				utility.SendJSONError(answer, w)
				return
			}
			publicKey := chain.Leaf.SubjectSignKey
			fmt.Println("publickey: ", publicKey)
			err = crypto.Verify(randomBytes, receivedSignature, publicKey, crypto.Ed25519)
			fmt.Println("VERIFY ERROR?: ", err)
			if err == nil {
				answer["pending"] = false

				// TODO: send gen folder
				answer["ia"], err = RemapASIDComputeNewGenFolder(as)
				if err != nil {
					// TODO: this is actually very bad, do we delete the AS entry here? what do we do?
					answer["error"] = true
					msg := fmt.Sprintf("ERROR in Coordinator: while mapping the ID, cannot generate a gen folder for the AS %s : %v", asID, err)
					answer["msg"] = msg
					log.Print(msg)
					utility.SendJSONError(answer, w)
					return
				}

				// fileName := "netsec.test.email@gmail.com_1-1001.tar.gz"
				// filePath := "/home/juan/scionLabConfigs/netsec.test.email@gmail.com_1-1001.tar.gz"
				// data, err := ioutil.ReadFile(filePath)
				// if err != nil {
				// 	// TODO

				// 	return
				// }
				// w.Header().Set("Content-Type", "application/gzip")
				// w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+fileName)
				// w.Header().Set("Content-Transfer-Encoding", "binary")
				// http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(data))
			}
		}
	} // if needed remapping
	err = utility.SendJSON(answer, w)
	if err != nil {
		log.Printf("Error during JSON marshaling: %v", err)
		s.Error500(w, err, "Error during JSON marshaling")
		return
	}
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
	as.Status = models.Remove
	cn.NeighborStatus = models.Remove
	cn.Status = models.Inactive
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
// Can remove a AS only if it is in the Active state.
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
	if as.Status == models.Active {
		if as.Type == models.Infrastructure {
			return false, nil, nil, errors.New("cannot remove infrastructure ASes")
		}
		cns, err := as.GetJoinConnectionInfo()
		if err != nil {
			return false, nil, nil, fmt.Errorf("error looking up connections: %v", err)
		}
		cns = models.OnlyCurrentConnections(cns)
		l := len(cns)
		if err != nil || l == 0 {
			return false, nil, nil, err
		}
		if l > 1 {
			return false, nil, nil, fmt.Errorf("AS %v has currently %v connections", asID, l)
		}
		// TODO: we support only one active connection per AS
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
	err = fmt.Errorf("the AS %v does not belong to the specified account", ia)
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

// RemapASIDComputeNewGenFolder creates a new gen folder using a valid remapped ID
// e.g. 17-ffaa:0:1 . This does not change IDs in the DB but recomputes topologies and certificates.
// After finishing, there will be a new tgz file ready to download using the mapped ID.
func RemapASIDComputeNewGenFolder(as *models.SCIONLabAS) (*addr.ISD_AS, error) {
	I, A := utility.MapOldIAToNewOne(as.ISD, as.ASID)
	if I == 0 || A == 0 {
		return nil, fmt.Errorf("Invalid source address to map: (%d, %d)", as.ISD, as.ASID)
	}
	ia := addr.ISD_AS{I: int(I), A: int(A)}
	// replace IDs in the AS entry, but don't save in DB:
	as.ISD = int(I)
	as.ASID = int(A)
	// retrieve connection:
	conns, err := as.GetJoinConnections()
	if err != nil {
		return nil, err
	}
	if len(conns) != 1 {
		err = fmt.Errorf("User AS should have only 1 connection. %s has %d", ia, len(conns))
		return nil, err
	}
	conn := conns[0]
	conn.RespondAP.AS = conn.GetRespondAS()
	asInfo, err := getSCIONLabASInfoFromDB(conn)
	fmt.Println("[DEBUG] ** 11")
	if err != nil {
		return nil, err
	}
	// finally, generate the gen folder:
	err = generateGenForAS(asInfo)
	if err != nil {
		return nil, err
	}

	return &ia, nil
}
