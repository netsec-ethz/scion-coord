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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/config"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/controllers/middleware"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/netsec-ethz/scion/go/lib/addr"
)

func UserPackagePath(email string) string {
	return filepath.Join(PackagePath, email)
}

type SCIONLabASController struct {
	controllers.HTTPController
}

type SCIONLabASInfo struct {
	IsNewUser   bool               // denotes whether this is a new user.
	UserEmail   string             // the email address of the user.
	IsVPN       bool               // denotes whether this is a VPN setup
	VPNServerIP string             // IP of the VPN server
	ISD         int                // ISD
	ASID        int                // asID of the SCIONLab AS
	IP          string             // the public IP address of the SCIONLab AS
	RemoteIA    string             // the SCIONLab AP the AS connects to
	RemoteIP    string             // the IP address of the SCIONLab AP it connects to
	RemoteBRID  int                // ID of the border router in the SCIONLab AP
	RemotePort  int                // Port of the BR in the SCIONLab AP
	SLAS        *models.SCIONLabAS // if exists, the DB object that belongs to this AS
	RemoteAS    *models.SCIONLabAS // the AP this AS connects to
}

// The main handler function to generates a SCIONLab AS for the given user.
// If successful, the front-end will initiate the downloading of the tarball.
func (s *SCIONLabASController) GenerateSCIONLabAS(w http.ResponseWriter, r *http.Request) {
	// Parse the arguments
	isVPN, scionLabASIP, userEmail, err := s.parseURLParameters(r)
	if err != nil {
		log.Printf("Error parsing the parameters: %v", err)
		s.BadRequest(w, err, "Error parsing the parameters")
		return
	}
	// check if there is already a create or update in progress
	canCreateOrUpdate, err := s.canCreateOrUpdate(userEmail)
	if err != nil {
		log.Printf("Error checking pending create or update for user %v: %v", userEmail, err)
		s.Error500(w, err, "Error checking pending create or update")
		return
	}
	if !canCreateOrUpdate {
		s.BadRequest(w, nil, "You have a pending operation. Please wait a few minutes.")
		return
	}
	// Target SCIONLab ISD and AS to connect to is determined by config file
	asInfo, err := s.getSCIONLabASInfo(scionLabASIP, userEmail, config.AP_IA, isVPN)
	if err != nil {
		log.Printf("Error getting SCIONLabASInfo: %v", err)
		s.Error500(w, err, "Error getting SCIONLabASInfo")
		return
	}
	// Remove all existing files from UserPackagePath
	// TODO(mlegner): May want to archive somewhere?
	os.RemoveAll(UserPackagePath(asInfo.UserEmail) + "/")
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
	if err = s.packageConfiguration(asInfo.UserEmail); err != nil {
		log.Printf("Error packaging SCIONLabAS configuration: %v", err)
		s.Error500(w, err, "Error packaging SCIONLabAS configuration")
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

// Parses the necessary parameters from the URL: whether or not this is a VPN setup,
// the user's email address, and the public IP of the user's machine.
func (s *SCIONLabASController) parseURLParameters(r *http.Request) (bool,
	string, string, error) {
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		return false, "", "", fmt.Errorf("Error getting the user session: %v", err)
	}
	userEmail := userSession.Email
	if err := r.ParseForm(); err != nil {
		return false, "", "", fmt.Errorf("Error parsing the form: %v", err)
	}
	slasIP := r.Form["scionLabASIP"][0]
	isVPN, _ := strconv.ParseBool(r.Form["isVPN"][0])
	if !isVPN && slasIP == "undefined" {
		return false, "", "", fmt.Errorf(
			"IP address cannot be empty for non-VPN setup. User: %v", userEmail)
	}
	return isVPN, slasIP, userEmail, nil
}

// Check if the user's AS is already in the process of being created or updated.
func (s *SCIONLabASController) canCreateOrUpdate(userEmail string) (bool, error) {
	slas, err := models.FindOneSCIONLabASByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			return true, nil
		} else {
			return false, err
		}
	}
	if (slas.Status == models.ACTIVE) || (slas.Status == models.INACTIVE) {
		return true, nil
	}
	return false, nil
}

// Populates and returns a SCIONLabASInfo struct, which contains the necessary information
// to create the SCIONLab AS configuration.
func (s *SCIONLabASController) getSCIONLabASInfo(scionLabASIP, userEmail, targetIA string,
	isVPN bool) (*SCIONLabASInfo, error) {
	var newUser bool
	var asID, brID int
	var ip, remoteIP, vpnIP string
	var cn models.ConnectionInfo
	// See if this user already has an AS
	slas, err := models.FindOneSCIONLabASByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			newUser = true
			asID, _ = s.getNewSCIONLabASID()
			brID = -1
		} else {
			return nil, fmt.Errorf("Error looking up SCIONLab AS for user %v: %v", userEmail, err)
		}
	} else {
		cn, err = slas.GetJoinConnectionInfoToAS(config.AP_IA)
		if err != nil {
			return nil, fmt.Errorf("Error looking up connections of SCIONLab AS for user %v: %v",
				userEmail, err)
		}
		newUser = false
		asID = slas.ASID
		brID = cn.BRID
	}
	log.Printf("AS ID given to the user %v: %v", userEmail, asID)

	ia, err := addr.IAFromString(targetIA)
	if err != nil {
		return nil, err
	}

	remoteAS, err := models.FindSCIONLabASByIAString(targetIA)
	if err != nil {
		return nil, fmt.Errorf("Error while retrieving AttachmentPoint %v: %v", targetIA, err)
	}

	// Different settings depending on whether it is a VPN or standard setup
	if isVPN {
		if !newUser && cn.IsVPN {
			ip = cn.LocalIP
		} else {
			ip, err = remoteAS.GetFreeVPNIP()
			if err != nil {
				return nil, err
			}
			log.Printf("New VPN IP to be assigned to user %v: %v", userEmail, ip)
		}
		remoteIP = remoteAS.AP.VPNIP
		vpnIP = remoteAS.PublicIP
	} else {
		ip = scionLabASIP
		remoteIP = remoteAS.PublicIP
		log.Printf("IP address of AttachementPoint = %v", remoteIP)
	}

	if brID < 0 {
		brID, err = remoteAS.GetFreeBRID()
		if err != nil {
			return nil, err
		}
		log.Printf("New BR ID to be assigned to user %v: %v", userEmail, brID)
	}

	return &SCIONLabASInfo{
		IsNewUser:   newUser,
		UserEmail:   userEmail,
		IsVPN:       isVPN,
		ISD:         ia.I,
		ASID:        asID,
		RemoteIA:    targetIA,
		IP:          ip,
		RemoteIP:    remoteIP,
		RemoteBRID:  brID,
		RemotePort:  remoteAS.GetPortNumberFromBRID(brID),
		VPNServerIP: vpnIP,
		SLAS:        slas,
		RemoteAS:    remoteAS,
	}, nil
}

// Updates the relevant database tables related to SCIONLab AS creation.
func (s *SCIONLabASController) updateDB(asInfo *SCIONLabASInfo) error {
	if asInfo.IsNewUser {
		// update the SCIONLabAS table
		newAS := models.SCIONLabAS{
			UserEmail: asInfo.UserEmail,
			StartPort: config.START_PORT,
			ISD:       asInfo.ISD,
			ASID:      asInfo.ASID,
			Status:    models.CREATE,
			Type:      models.VM,
		}
		if !asInfo.IsVPN {
			newAS.PublicIP = asInfo.IP
		}
		if err := newAS.Insert(); err != nil {
			return fmt.Errorf("Error inserting new SCIONLabAS %v for user %v: %v",
				newAS.String(), asInfo.UserEmail, err)
		}
		log.Printf("New SCIONLab AS AS successfully created. User: %v new AS: %v", asInfo.UserEmail,
			newAS.String())
		// update the Connections table
		newCn := models.Connection{
			JoinIP:        asInfo.IP,
			RespondIP:     asInfo.RemoteIP,
			JoinAS:        &newAS,
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
				asInfo.UserEmail, err)
		}
	} else {
		// Update the Connections Table
		cn, err := asInfo.SLAS.GetJoinConnectionInfoToAS(asInfo.RemoteIA)
		if err != nil {
			return fmt.Errorf("Error finding existing connection of user %v: %v",
				asInfo.UserEmail, err)

		}
		cn.IsVPN = asInfo.IsVPN
		if cn.Status == models.INACTIVE {
			asInfo.SLAS.Status = models.CREATE
		} else {
			asInfo.SLAS.Status = models.UPDATE
		}
		cn.NeighborStatus = asInfo.SLAS.Status
		cn.Status = models.ACTIVE
		if err := asInfo.SLAS.UpdateASAndConnection(&cn); err != nil {
			return fmt.Errorf("Error updating database tables for user %v: %v",
				asInfo.UserEmail, err)
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
	return filepath.Join(TempPath, asInfo.UserEmail+"_topology.json")
}

// Generates the topology file for the SCIONLab AS AS. It uses the template file
// simple_config_topo.tmpl under templates folder in order to populate and generate the
// JSON file.
func (s *SCIONLabASController) generateTopologyFile(asInfo *SCIONLabASInfo) error {
	log.Printf("Generating topology file for SCIONLab AS")
	t, err := template.ParseFiles("templates/simple_config_topo.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing topology template config for user %v: %v",
			asInfo.UserEmail, err)
	}
	f, err := os.Create(asInfo.topologyFile())
	if err != nil {
		return fmt.Errorf("Error creating topology file config for user %v: %v", asInfo.UserEmail,
			err)
	}

	// Topo file parameters
	data := map[string]string{
		"IP":           asInfo.IP,
		"BIND_IP":      asInfo.SLAS.BindIP(asInfo.IsVPN, asInfo.IP),
		"ISD_ID":       strconv.Itoa(asInfo.ISD),
		"AS_ID":        strconv.Itoa(asInfo.ASID),
		"TARGET_ISDAS": asInfo.RemoteIA,
		"REMOTE_ADDR":  asInfo.RemoteIP,
		"REMOTE_PORT":  strconv.Itoa(asInfo.RemotePort),
	}
	if err = t.Execute(f, data); err != nil {
		return fmt.Errorf("Error executing topology template file for user %v: %v",
			asInfo.UserEmail, err)
	}
	f.Close()
	return nil
}

// Creates the local gen folder of the SCIONLab AS AS. It calls a Python wrapper script
// located under the python directory. The script uses SCION's and SCION-WEB's library
// functions in order to generate the certificate, AS keys etc.
func (s *SCIONLabASController) generateLocalGen(asInfo *SCIONLabASInfo) error {
	log.Printf("Creating gen folder for SCIONLab AS")
	asID := strconv.Itoa(asInfo.ASID)
	isdID := strconv.Itoa(asInfo.ISD)
	userEmail := asInfo.UserEmail
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isdID, asID,
		userEmail)
	cmd := exec.Command("python3", localGenPath,
		"--topo_file="+asInfo.topologyFile(), "--user_id="+userEmail,
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
		return fmt.Errorf("Generate local gen command could not start for user %v: %v",
			asInfo.UserEmail, err)
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT generateLocalGen: %s\n", stdOutput)
	fmt.Printf("ERROUT generateLocalGen: %s\n", errOutput)
	return nil
}

// Packages the SCIONLab AS configuration as a tarball and returns the name of the
// generated file.
func (s *SCIONLabASController) packageConfiguration(userEmail string) error {
	log.Printf("Packaging SCIONLab AS")
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
				return fmt.Errorf("Failed to copy files for user %v: src: %v, dst: %v, %v",
					userEmail, src, dst, err)
			}
		}
	}
	cmd := exec.Command("tar", "zcvf", userEmail+".tar.gz", userEmail)
	cmd.Dir = PackagePath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to create SCIONLabAS tarball for user %v: %v", userEmail, err)
	}
	return nil
}

// API end-point to serve the generated SCIONLab AS configuration tarball.
func (s *SCIONLabASController) ReturnTarball(w http.ResponseWriter, r *http.Request) {
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Forbidden(w, err, "Error getting the user session")
		return
	}
	slas, err := models.FindOneSCIONLabASByUserEmail(userSession.Email)
	if err != nil || slas.Status == models.INACTIVE || slas.Status == models.REMOVE {
		log.Printf("No active configuration found for user %v\n", userSession.Email)
		s.BadRequest(w, nil, "No active configuration found for user %v",
			userSession.Email)
		return
	}

	fileName := userSession.Email + ".tar.gz"
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
	_, userSession, err := middleware.GetUserSession(r)
	if err != nil {
		log.Printf("Error getting the user session: %v", err)
		s.Error500(w, err, "Error getting the user session")
	}
	userEmail := userSession.Email

	// check if there is an active AS which can be removed
	canRemove, slas, cn, err := s.canRemove(userEmail)
	if err != nil {
		log.Printf("Error checking if your AS can be removed for user %v: %v", userEmail, err)
		s.Error500(w, err, "Error checking if AS can be removed")
		return
	}
	if !canRemove {
		s.BadRequest(w, nil, "You currently do not have an active SCIONLab AS.")
		return
	}
	slas.Status = models.REMOVE
	cn.NeighborStatus = models.REMOVE
	cn.Status = models.INACTIVE
	if err := slas.UpdateASAndConnection(cn); err != nil {
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
func (s *SCIONLabASController) canRemove(userEmail string) (bool, *models.SCIONLabAS,
	*models.ConnectionInfo, error) {
	slas, err := models.FindOneSCIONLabASByUserEmail(userEmail)
	if err != nil {
		if err == orm.ErrNoRows {
			return false, nil, nil, nil
		} else {
			return false, nil, nil, err
		}
	}
	if slas.Status == models.ACTIVE {
		cn, err := slas.GetJoinConnectionInfoToAS(config.AP_IA)
		if err != nil {
			return false, nil, nil, err
		}
		return true, slas, &cn, nil
	}
	return false, nil, nil, nil
}
