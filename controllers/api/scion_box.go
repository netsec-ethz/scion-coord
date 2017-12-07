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
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"text/template"
	"time"

	"github.com/astaxie/beego/orm"
	"github.com/netsec-ethz/scion-coord/controllers"
	"github.com/netsec-ethz/scion-coord/models"
	"github.com/netsec-ethz/scion-coord/utility"
	"github.com/netsec-ethz/scion-coord/utility/geolocation"
	"github.com/netsec-ethz/scion-coord/utility/topologyAlgorithm"
	"github.com/netsec-ethz/scion/go/lib/addr"
)

type SCIONBoxController struct {
	controllers.HTTPController
}

// API Endpoint which the box calls when it starts up and has no credentials and
// gen folder.
// Receives a Post Request with json:
// {IPAddress: '1.2.3.4', MacAddress: '55:13:56:f4:1f:26', openPorts: 10, startPort: 50000}
// If Box is new (never connected before) look through Database for potential neighbors
// and send a List of them to the Box and credentials to the box.
// Example Reply: {PotentialNeighbors:[
//			{AS_ID: "1",ISD_ID: "1", IP: "135.251.53.1"},
//			{AS_ID: "6",ISD_ID: "1", IP: "13.2.53.1"}
// 			],
//			ID: 	"aödfq3r24köy842",
//			SECRET:	"d2398cjkw42" ,
//			IP:	"21.4.65.2"
//			Usermail: "pmao@student.eth.ch",
//			ISD_ID: 1,
//		    }
func (s *SCIONBoxController) InitializeBox(w http.ResponseWriter, r *http.Request) {
	// Parse the arguments
	_, ip, external_ip, mac, openPorts, startPort, err := s.parseRequest(r)
	if err != nil {
		log.Printf("Error parsing parameters and source IP: %v", err)
		s.BadRequest(err, w, r)
		return
	}
	// Retrieve the SCIONBox information
	sb, err := models.FindSCIONBoxByMAC(mac)
	if err != nil {
		log.Printf("Error retrieving the box info: %v, %v", mac, err)
		s.Error500(err, w, r)
		return
	}
	// Update Connectivity info of the Box
	sb.StartPort = startPort
	sb.OpenPorts = openPorts
	sb.Update()
	if err != nil {
		log.Printf("Error updating the box connection info: %v, %v", openPorts, err)
		s.Error500(err, w, r)
		return
	}
	if openPorts == 0 {
		log.Printf("no Free UDP ports for Border Routers !: %v, %v", openPorts, err)
		s.BadRequest(fmt.Errorf("No open UDP ports!"),
			w, r)
		return
	}
	// Check if the box already exists
	slas, err := models.FindSCIONLabASByIAInt(sb.ISD, sb.AS)
	if err != nil {
		if err == orm.ErrNoRows {
			s.initializeNewBox(sb, external_ip, mac, w, r)
		} else {
			log.Printf("Error retrieving ScionlabAS info: %v, %v", mac, err)
			s.Error500(err, w, r)
		}
	} else {
		s.initializeOldBox(sb, slas, ip, mac, w, r)
	}
}

// Checks if the Box needs an update
func (s *SCIONBoxController) initializeNewBox(sb *models.SCIONBox, ip string, mac string,
	w http.ResponseWriter, r *http.Request) {
	// Create the Usercredential path
	//os.Mkdir(UserPackagePath(sb.UserEmail), 0777)
	// Check if the box needs an update
	if sb.UpdateRequired {
		log.Printf("Shipped box needs an update !: %v, %v", mac, sb.UserEmail)
		// TODO Update the box !
		sb.UpdateRequired = false
		sb.Update()
	} else {
		s.sendPotentialNeighbors(sb, ip, mac, w, r)
	}
}

// Run through the steps required to connect a previously connected Box.
func (s *SCIONBoxController) initializeOldBox(sb *models.SCIONBox, slas *models.SCIONLabAS,
	ip string, mac string, w http.ResponseWriter, r *http.Request) {
	BoxStatus := slas.Status
	if BoxStatus == models.UPDATE {
		log.Printf("Box that needs to be updated has requested an init box!: %v, %v",
			mac, BoxStatus)
		slas.Status = models.INACTIVE
		slas.Update()
		// TODO Update the box !
	} else {
		log.Printf("Previously connected Box needs a gen folder!: %v, %v", mac, sb.UserEmail)
		// Check if connection has changed
		// If the IP address is still the same simply serve the gen folder again,
		// Otherwise disconnect the old box and connect the box like a new box.
		if slas.PublicIP == ip {
			// Generate necessary files and send them to the Box
			os.RemoveAll(userPackagePath(slas.UserMail))
			os.Remove(filepath.Join(BoxPackagePath, slas.UserMail+".tar.gz"))
			if err := s.generateGen(slas); err != nil {
				s.Error500(err, w, r)
				return
			}
			s.serveGen(slas.UserMail, w, r)
		} else {
			if err := s.disconnectBox(sb, slas, false); err != nil {
				log.Printf("Error disconnecting box, %v, source_ip: %v, mac_address %v",
					err, ip, mac)
			}
			s.sendPotentialNeighbors(sb, ip, mac, w, r)
		}
	}
}

type InitRequest struct {
	MacAddress string
	IPAddress  string
	OpenPorts  int // Number of open ports starting from StartPort
	StartPort  int
}

// Receive a Post request with json: {IPAddress: 'string', MacAddress: 'string', OpenPorts: int, StartPort: int}
func (s *SCIONBoxController) parseRequest(r *http.Request) (bool, string, string, string, int, int, error) {
	var isNAT bool
	var request InitRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		return false, "", "", "", 0, 0, err
	}
	mac_address := request.MacAddress
	ip := request.IPAddress
	sourceIP, err := s.getSourceIP(r)
	if err != nil {
		return false, "", "", "", 0, 0, err
	}
	// Check if the box is behind a NAT or not
	var external_ip string
	if utility.IPCompare(sourceIP, ip) == 0 {
		isNAT = false
		external_ip = ip
	} else {
		isNAT = true
		external_ip = sourceIP
	}
	// parse the Connection results
	openPorts := request.OpenPorts
	startPort := request.StartPort
	log.Printf("isNAT: %t, source_ip: %v, ip_address: %v, Connections: %v", isNAT, sourceIP, ip, openPorts)
	return isNAT, ip, external_ip, mac_address, openPorts, startPort, nil
}

type initReply struct {
	PotentialNeighbors []topologyAlgorithm.Neighbor
	IP                 string
	ID                 string
	SECRET             string
	UserMail           string
	ISD_ID             int
}

// Sends a list of potential neighbors and credentials to the SCION-Box.
func (s *SCIONBoxController) sendPotentialNeighbors(sb *models.SCIONBox, ip string, mac string, w http.ResponseWriter, r *http.Request) {
	// run ip geolocation
	pns, isd, err := s.getPotentialNeighbors(ip, mac)
	if err != nil {
		log.Printf("Error looking for potential neighbors, %v, source_ip: %v, mac_address %v", err, ip, mac)
		s.Error500(err, w, r)
		return
	}
	// update the SCIONBox database
	sb.ISD = isd
	if err := sb.Update(); err != nil {
		log.Printf("Error updating scionbox database, %v", err)
		s.Error500(err, w, r)
		return
	}
	// build the reply json with IP, ID and SECRET
	id, secret, err := s.getCredentialsByEmail(sb.UserEmail)
	reply := initReply{
		PotentialNeighbors: pns,
		IP:                 ip,
		ID:                 id,
		SECRET:             secret,
		UserMail:           sb.UserEmail,
		ISD_ID:             isd,
	}
	s.JSON(reply, w, r)
	log.Printf("Sending pot neighbor %v", reply)
}

// Returns a list of potential Neighbors: active attachment point SCIONLabAses in the same ISD
// Also returns the assigned ISD
func (s *SCIONBoxController) getPotentialNeighbors(ip string, mac string) ([]topologyAlgorithm.Neighbor, int, error) {
	// run IP geolocation
	var potentialNeighbors []topologyAlgorithm.Neighbor
	country, continent, err := geolocation.IP_geolocation(ip)
	if err != nil {
		return potentialNeighbors, -1, err
	}
	log.Printf("New Box is in %s, %s,", continent, country)
	// check in which ISD the box is.
	isd, err := geolocation.Location2Isd(country, continent)
	if err != nil {
		return potentialNeighbors, -1, err
	}
	// look trough database for ASes in the same isd
	pns, err := models.FindPotentialNeighbors(isd)
	if err != nil {
		return potentialNeighbors, -1, err
	}
	for _, pn := range pns {
		newnb := topologyAlgorithm.Neighbor{
			ISD: pn.ISD,
			AS:  pn.AS,
			IP:  pn.PublicIP,
			BW:  -1,
			RTT: -1,
		}
		potentialNeighbors = append(potentialNeighbors, newnb)
	}
	return potentialNeighbors, isd, nil
}

// Returns the account id and secret of a user
func (s *SCIONBoxController) getCredentialsByEmail(userEmail string) (string, string, error) {
	user, err := models.FindUserByEmail(userEmail)
	if err != nil {
		fmt.Errorf("Error looking for user %v", err)
		return "", "", err
	}
	account := user.Account
	return account.AccountId, account.Secret, nil
}

type ConnectQuery struct {
	Neighbors []topologyAlgorithm.Neighbor
	IP        string
	UserMail  string
}

// Function called by the Box after it has finished BW & RTT tests.
// Recevies a Post Request with json:
//	{PotentialNeighbors:[
//			{AS_ID: "1",ISD_ID: "1", IP: "135.251.53.1", BW: 1.2, RTT: 0.00004},
//			{AS_ID: "6",ISD_ID: "1", IP: "13.2.53.1", BW: 22, RTT: 0.00045}
// 						],
//	 IP : "1.2.3.4", UserMail: "philipp@mail.eth"
//	}
// Runs the topology algorithm to choose Neighbors,
// Updates the database, generates necessary files and sends them to the Box
func (s *SCIONBoxController) ConnectNewBox(w http.ResponseWriter, r *http.Request) {
	// Parse the request
	var req ConnectQuery
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.BadRequest(err, w, r)
		return
	}
	// Retrive scionbox object using email
	sb, err := models.FindSCIONBoxByEMail(req.UserMail)
	if err != nil {
		log.Printf("Error looking for Scionbox, %v, %v", err, req.UserMail)
		s.Error500(err, w, r)
		return
	}
	// Choose the neigbhbors of the box
	neighbors := topologyAlgorithm.ChooseNeighbors(req.Neighbors, sb.OpenPorts)
	if len(neighbors) == 0 {
		log.Printf("Error no Neighbors for ScionBox, %v, ", req.UserMail)
		s.Error500(fmt.Errorf("No Neighbors!"), w, r)
		return
	}
	// Get the Reqeust IP
	ip := req.IP
	isd := sb.ISD
	// Update the Database with the new ScionLabAS
	slas, err := s.updateDBnewSB(sb, neighbors, isd, ip)
	if err != nil {
		log.Printf("Updating the Database, %v", err)
		s.Error500(err, w, r)
		return
	}
	// Generate necessary files and send them to the Box
	os.RemoveAll(userPackagePath(slas.UserMail))
	os.Remove(filepath.Join(BoxPackagePath, slas.UserMail+".tar.gz"))
	if err := s.generateGen(slas); err != nil {
		s.Error500(err, w, r)
		return
	}
	s.serveGen(slas.UserMail, w, r)
}

// this function inserts a new SCIONBox into the database
func (s *SCIONBoxController) updateDBnewSB(sb *models.SCIONBox, neighbors []topologyAlgorithm.Neighbor, isd int, ip string) (*models.SCIONLabAS, error) {
	user, err := models.FindUserByEmail(sb.UserEmail)
	if err != nil {
		return nil, fmt.Errorf("Error finding the user by email %v: %v", sb.UserEmail, err)
	}
	as, err := s.getNewSCIONBoxASID(isd)
	if err != nil {
		return nil, fmt.Errorf("Error looking for new AS-ID %v: %v", sb.UserEmail, err)
	}
	newAs := &models.As{
		Isd:     isd,
		As:      as,
		Core:    false,
		Account: user.Account,
		Created: time.Now().UTC(),
	}
	if err = newAs.Insert(); err != nil {
		return nil, fmt.Errorf("Error inserting new AS: %v User: %v, %v", newAs.String(), user,
			err)
	}
	account := user.Account
	account.ASes = append(account.ASes, newAs)
	if err = account.Upsert(); err != nil {
		return nil, fmt.Errorf("Error upserting account: %v User: %v, %v", newAs.String(), user,
			err)
	}
	// Box has now VPN server
	newAP := &models.AttachmentPoint{
		VPNIP:      "0.0.0.0",
		StartVPNIP: "0.0.0.0",
		EndVPNIP:   "0.0.0.0",
	}
	if err = newAP.Insert(); err != nil {
		return nil, fmt.Errorf("Error inserting new AttachmentPoint Info. User: %v, %v", newAP, err)
	}
	newSlas := &models.SCIONLabAS{
		UserMail:  sb.UserEmail,
		PublicIP:  ip,
		StartPort: sb.StartPort,
		ISD:       isd,
		AS:        as,
		Status:    models.CREATE,
		Type:      models.BOX,
		AP:        newAP,
	}
	if err = newSlas.Insert(); err != nil {
		return nil, fmt.Errorf("Error inserting new SCIONLabAS info. User: %v, %v", newSlas, err)
	}
	// Start the goroutine which updates the status
	go s.checkHBStatus(isd, as)
	// Update the Box information
	sb.AS = as
	if err = sb.Update(); err != nil {
		return nil, fmt.Errorf("Error Updating SCIONBox info. %v, %v", sb, err)
	}
	// generate Connection between SCIONLabAs the two ASes.
	for i, neighbor := range neighbors {
		nbSlas, err := models.FindSCIONLabASByIAInt(neighbor.ISD, neighbor.AS)
		if err != nil {
			log.Printf("Neighbor Slas not found %v ", err)
			continue
		}
		acceptId := s.findLowestBRId(nbSlas)
		// Connection for the two ASes
		cn := models.Connection{
			JoinIP:        ip,
			RespondIP:     nbSlas.PublicIP,
			JoinAS:        newSlas,
			RespondAP:     nbSlas.AP,
			JoinBRID:      i + 1,
			RespondBRID:   acceptId,
			Linktype:      models.PARENT,
			IsVPN:         false,
			JoinStatus:    models.CREATE,
			RespondStatus: models.CREATE,
		}
		if err = cn.Insert(); err != nil {
			return nil, fmt.Errorf("Error Inserting Connection info. %v", cn)
		}
	}
	return newSlas, nil
}

// Generate the gen folder
func (s *SCIONBoxController) generateGen(slas *models.SCIONLabAS) error {
	if err := s.generateTopologyFile(slas); err != nil {
		log.Printf("Error generating topology File: %v", err)
		return err
	}
	if err := s.generateGenFolder(slas); err != nil {
		log.Printf("Error generating gen Folder: %v", err)
		return err
	}
	if err := s.generateCredentialsFile(slas); err != nil {
		log.Printf("Error generating credentials file: %v", err)
		return err
	}
	return nil
}

func (s *SCIONBoxController) serveGen(userMail string, w http.ResponseWriter, r *http.Request) {
	if err := s.packageGenFolder(userMail); err != nil {
		log.Printf("Error packaging gen folder: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Wait to make sure a previous version is not served
	time.Sleep(1000 * time.Millisecond)
	// serve the packaged gen folder to the box
	fileName := userMail + ".tar.gz"
	filePath := filepath.Join(BoxPackagePath, fileName)
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading tar file: %v", err)
		s.Error500(err, w, r)
		return
	}
	// Send the gzip file to the Box
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=scion_lab_"+fileName)
	w.Header().Set("Content-Transfer-Encoding", "binary")
	http.ServeContent(w, r, fileName, time.Now(), bytes.NewReader(data))
}

//get the source IP from a http request
func (s *SCIONBoxController) getSourceIP(r *http.Request) (string, error) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	//userIP := net.ParseIP(ip)
	return ip, nil
}

//  Provides a new AS ID for the newly connected SCION box.
func (s *SCIONBoxController) getNewSCIONBoxASID(isd int) (int, error) {
	ases, err := models.FindSCIONLabAsesByISD(isd)
	if err != nil {
		return -1, err
	}
	// Base AS ID for SCION boxes starts from 2000
	asID := 2000
	for _, as := range ases {
		if as.AS > asID {
			asID = as.AS
		}
	}
	return asID + 1, nil
}

// Find the lowest available Port number
func (s *SCIONBoxController) findLowestBRId(slas *models.SCIONLabAS) int {
	var newIdFound = false
	var ID = 1
	var idFound bool
	cns, _ := slas.GetConnectionInfo()
	for !newIdFound {
		idFound = false
		for _, cn := range cns {
			if cn.Status != models.REMOVED {
				if cn.BRID == ID {
					idFound = true
					break
				}
			}
		}
		if idFound {
			ID++
		} else {
			newIdFound = true
		}
	}
	return ID
}

// Generates the path to the temporary topology file
func (s *SCIONBoxController) topologyFile(slas *models.SCIONLabAS) string {
	return filepath.Join(TempPath, slas.UserMail+"SCIONBox_topology.json")
}

// Generates the topology file for the SCIONLabAS. It uses the template file
// simple_box_config_topo.tmpl under templates folder in order to populate and generate the
// JSON file.
func (s *SCIONBoxController) generateTopologyFile(slas *models.SCIONLabAS) error {
	log.Printf("Generating topology file for SCIONLab Box")
	t, err := template.ParseFiles("templates/simple_box_config_topo.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing topology template config. User: %v, %v",
			slas.UserMail, err)
	}
	f, err := os.Create(s.topologyFile(slas))
	if err != nil {
		return fmt.Errorf("Error creating topology file config. User: %v, %v", slas.UserMail,
			err)
	}
	type Br struct {
		ISD_ID       string
		AS_ID        string
		REMOTE_ADDR  string
		REMOTE_PORT  string
		LOCAL_PORT   string
		TARGET_ISDAS string
		IP           string
		BIND_IP      string
		BIND_PORT    string
		COMMA        string
		ID           string
		LINK_TYPE    string
	}
	type Topo struct {
		ISD_ID string
		AS_ID  string
		IP     string
		BR     []Br
	}
	var borderrouters []Br
	brs, err := slas.GetConnectionInfo()
	if err != nil {
		return fmt.Errorf("Error retrivieng brs for slas. User: %v, %v", slas.UserMail,
			err)
	}
	for i, br := range brs {
		log.Printf("adding BR objects in topology genareation")
		ia := addr.ISD_AS{
			I: br.NeighborISD,
			A: br.NeighborAS,
		}
		linktype := GetLinktype(br.Linktype)
		bro := Br{
			ISD_ID:       strconv.Itoa(slas.ISD),
			AS_ID:        strconv.Itoa(slas.AS),
			REMOTE_ADDR:  br.NeighborIP,
			REMOTE_PORT:  strconv.Itoa(br.RemotePort),
			LOCAL_PORT:   strconv.Itoa(br.LocalPort),
			TARGET_ISDAS: ia.String(),
			IP:           slas.PublicIP,
			BIND_IP:      slas.PublicIP,
			BIND_PORT:    strconv.Itoa(br.LocalPort),
			ID:           strconv.Itoa(br.BRID),
			LINK_TYPE:    linktype,
		}
		// if last neighbor do not add the Comma to the end
		if i == len(brs)-1 {
			bro.COMMA = ""
		} else {
			bro.COMMA = ","
		}
		borderrouters = append(borderrouters, bro)
	}
	topo := Topo{
		ISD_ID: strconv.Itoa(slas.ISD),
		AS_ID:  strconv.Itoa(slas.AS),
		BR:     borderrouters,
		IP:     slas.PublicIP,
	}
	if err = t.Execute(f, topo); err != nil {
		return fmt.Errorf("Error executing topology template file. User: %v, %v",
			slas.UserMail, err)
	}
	f.Close()
	return nil
}

func (s *SCIONBoxController) generateCredentialsFile(slas *models.SCIONLabAS) error {
	log.Printf("Generating credentials file for SCIONBox")
	t, err := template.ParseFiles("templates/box_credentials.tmpl")
	if err != nil {
		return fmt.Errorf("Error parsing credentials template config. User: %v, %v",
			slas.UserMail, err)
	}
	f, err := os.Create(filepath.Join(userPackagePath(slas.UserMail), "box_credentials.conf"))
	if err != nil {
		return fmt.Errorf("Error creating credentials file config. User: %v, %v", slas.UserMail,
			err)
	}
	type Cr struct {
		ID       string
		SECRET   string
		IP       string
		ISD      int
		AS       int
		USERMAIL string
	}
	// find Account
	id, secret, err := s.getCredentialsByEmail(slas.UserMail)
	if err != nil {
		fmt.Errorf("Error looking for credentials %v", err)
	}
	cr := Cr{
		ID:       id,
		SECRET:   secret,
		IP:       slas.PublicIP,
		ISD:      slas.ISD,
		AS:       slas.AS,
		USERMAIL: slas.UserMail,
	}
	if err = t.Execute(f, cr); err != nil {
		return fmt.Errorf("Error executing credentials template file. User: %v, %v",
			slas.UserMail, err)
	}
	return nil
}

// Creates the local gen folder of the SCIONLabAS . It calls a Python wrapper script
// located under the python directory. The script uses SCION's and SCION-WEB's library
// functions in order to generate the certificate, AS keys etc.
func (s *SCIONBoxController) generateGenFolder(slas *models.SCIONLabAS) error {
	log.Printf("Creating gen folder for SCIONBox")
	asID := strconv.Itoa(slas.AS)
	isdID := strconv.Itoa(slas.ISD)
	userEmail := slas.UserMail
	CoreCredentialsPath := ISDCoreCredentialsPath(isdID)
	log.Printf("Calling create local gen. ISD-ID: %v, AS-ID: %v, UserEmail: %v", isdID, asID,
		userEmail)
	cmd := exec.Command("python3", localGenPath,
		"--topo_file="+s.topologyFile(slas), "--user_id="+userEmail,
		"--joining_ia="+isdID+"-"+asID,
		"--core_ia="+isdID+"-1",
		"--core_sign_priv_key_file="+getCoreSigKeyPath(CoreCredentialsPath),
		"--core_cert_file="+getCoreCertPath(CoreCredentialsPath, isdID),
		"--trc_file="+getCoreTrcFilePath(CoreCredentialsPath, isdID),
		"--package_path="+BoxPackagePath)
	os.Setenv("PYTHONPATH", pythonPath+":"+scionPath+":"+scionWebPath)
	cmd.Env = os.Environ()
	cmdOut, _ := cmd.StdoutPipe()
	cmdErr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Generate local gen command could not start. User: %v, %v",
			slas.UserMail, err)
	}
	// read stdout and stderr
	stdOutput, _ := ioutil.ReadAll(cmdOut)
	errOutput, _ := ioutil.ReadAll(cmdErr)
	fmt.Printf("STDOUT generateLocalGen: %s\n", stdOutput)
	fmt.Printf("ERROUT generateLocalGen: %s\n", errOutput)
	return nil
}

// Packages the gen folder and credential file
func (s *SCIONBoxController) packageGenFolder(userEmail string) error {
	log.Printf("Packaging gen Folder")
	cmd := exec.Command("tar", "zcvf", userEmail+".tar.gz", userEmail)
	cmd.Dir = BoxPackagePath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to create SCIONLabAS tarball. User: %v, %v", userEmail, err)
	}
	return nil
}

// Updates the relevant database tables related to removing a SCION Box from the network.
func (s *SCIONBoxController) disconnectBox(sb *models.SCIONBox, slas *models.SCIONLabAS, hasGen bool) error {
	// Set the status of all connections to REMOVE
	cns, err := slas.GetConnectionInfo()
	if err != nil {
		return err
	}
	for _, cn := range cns {
		cn.Status = models.REMOVE
		err := slas.UpdateDBConnection(cn)
		if err != nil {
			return err
		}
		// If the Box has no gen Folder set own Connection Status to REMOVED
		if !hasGen {
			cn.Status = models.REMOVED
			err := slas.UpdateDBConnection(cn)
			if err != nil {
				return err
			}
		}
	}
	// Update the ScionLabAS Status
	slas.Status = models.REMOVE
	if err := slas.Update(); err != nil {
		return err
	}
	// Update the ScionBox ISD-AS
	sb.ISD = 0
	sb.AS = 0
	if err := sb.Update(); err != nil {
		return err
	}
	return nil
}

// struct for the heartbeat Query just enough info
type CurrentCn struct {
	NeighborIA string
	NeighborIP string
	RemotePort int
}

type IA struct {
	ISD         int
	AS          int
	Connections []CurrentCn
}

type HeartBeatQuery struct {
	IAList   []IA
	UserMail string
	IP       string
	Time     float64
}

type HBResponse struct {
	IAList []ResponseIA
}

type ResponseIA struct {
	ISD         int
	AS          int
	Connections []models.ConnectionInfo
}

// Heartbeat function
// TODO Receive some status information about the box (reachibility of neighbors ? )
func (s *SCIONBoxController) HeartBeatFunction(w http.ResponseWriter, r *http.Request) {
	// get the account tied to the box
	// Parse the received info
	var req HeartBeatQuery
	log.Printf("new HB Query")
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding JSON: %v, %v", r.Body, err)
		s.Error500(err, w, r)
		return
	}
	ip := req.IP
	var needGen = false
	var slasList []*models.SCIONLabAS
	for _, ia := range req.IAList {
		slas, err := models.FindSCIONLabASByIAInt(ia.ISD, ia.AS)
		if err != nil {
			if err == orm.ErrNoRows {
				// no row found AS is not a SCIONLabAS
				continue
			} else {
				log.Printf("no SCIONLabAS found in HB, %v %v", ia, err)
				s.Error500(err, w, r)
				return
			}
		}
		// check if box needs an update
		if slas.Status == models.UPDATE {
			slas.Status = models.INACTIVE
			slas.Update()
			// TODO Update the box !
			return
		}
		needGen, err = s.HBCheckIP(slas, ip, ia, r)
		if err != nil {
			log.Printf("Error running IP checks in HB: %v,", err)
			s.Error500(err, w, r)
			return
		}
		// Send connections in the Database to the Box
		slasList = append(slasList, slas)
	}
	if needGen {
		// TODO generate updated gen folder for all ASes
		// Remove old gen folders/ packages
		os.RemoveAll(userPackagePath(slasList[0].UserMail))
		os.Remove(filepath.Join(BoxPackagePath, slasList[0].UserMail+".tar.gz"))
		for _, slas := range slasList {
			// Generate necessary files and send them to the Bo
			if err := s.generateGen(slas); err != nil {
				s.Error500(err, w, r)
				return
			}
		}
		s.serveGen(slasList[0].UserMail, w, r)
	} else {
		var iaList []ResponseIA
		for _, slas := range slasList {
			cns, err := slas.GetConnectionInfo()
			log.Printf("Got Connection Info")
			if err != nil {
				log.Printf("Error retrivieng brs %v", err)
				s.Error500(err, w, r)
				return
			}
			slas.Status = models.ACTIVE
			if err := slas.Update(); err != nil {
				log.Printf("Error updating slas %v", err)
				s.Error500(err, w, r)
				return
			}
			ia := ResponseIA{
				ISD:         slas.ISD,
				AS:          slas.AS,
				Connections: cns,
			}
			iaList = append(iaList, ia)
		}
		hbResponse := HBResponse{
			IAList: iaList,
		}
		s.JSON(hbResponse, w, r)
	}
}

// Check if IP address has changed
func (s *SCIONBoxController) HBCheckIP(slas *models.SCIONLabAS, ip string, ia IA, r *http.Request) (bool, error) {
	var needGen = false
	if utility.IPCompare(ip, slas.PublicIP) != 0 {
		// The IP address of the Box has changed update the DB
		if err := s.HBChangedIP(slas, ip); err != nil {
			return needGen, fmt.Errorf("Error updating the Box Connectons with changed IP: %v",
				err)
		}
		needGen = true
		return needGen, nil
	} else {
		// Update the database using the list of received Neighbors
		if err := s.updateDBConnections(slas, ia.Connections); err != nil {
			return needGen, fmt.Errorf("Error updating the Box Connectons: %v",
				err)
		}
	}
	return needGen, nil
}

// IP address of the Box has changed --> Update the Database
func (s *SCIONBoxController) HBChangedIP(slas *models.SCIONLabAS, ip string) error {
	// Update the ScionLabAS database
	slas.PublicIP = ip
	if err := slas.Update(); err != nil {
		return fmt.Errorf("Error updating the Box Status: %v",
			err)
	}
	// Update the Connection database
	cns, err := slas.GetConnectionInfo()
	log.Printf("Connections: %v", cns)
	if err != nil {
		return fmt.Errorf("Error retrieving Box Connections: %v",
			err)
	}
	// Update the connections
	for _, cn := range cns {
		cn.Status = models.UPDATE
		if err := slas.UpdateDBConnection(cn); err != nil {
			return fmt.Errorf("Error updating the Connection: %v",
				err)
		}
	}
	return nil
}

// Update the database with the list of the boxes current neighbors received from the box
// NEW -> UP if borderrouter in the list
// DELETE -> remove from db if borderrouter in the list
func (s *SCIONBoxController) updateDBConnections(slas *models.SCIONLabAS, neighbors []CurrentCn) error {
	cns, err := slas.GetConnectionInfo()
	if err != nil {
		return err
	}
	for _, cn := range cns {
		if cn.Status == models.CREATE {
			found := findCnInNbs(cn, neighbors)
			if found {
				cn.Status = models.ACTIVE
			}
		}
		if cn.Status == models.UPDATE {
			found := findCnInNbs(cn, neighbors)
			if found {
				cn.Status = models.ACTIVE
			}
		}
		if cn.Status == models.REMOVE {
			found := findCnInNbs(cn, neighbors)
			if !found {
				cn.Status = models.REMOVED
			}
		}
		err := slas.UpdateDBConnection(cn)
		if err != nil {
			return err
		}
	}
	return nil
}

func findCnInNbs(cn models.ConnectionInfo, neighbors []CurrentCn) bool {
	var found = false
	for _, nb := range neighbors {
		if nb.NeighborIP == cn.NeighborIP {
			if nb.RemotePort == cn.RemotePort {
				found = true
				break
			}
		}
	}
	return found
}

// goroutine that periodically checks the time between the time the SLAS called the Heartbeat API
// if the time is 10 times the HBPERIOD status is set to INACTIVE
func (s *SCIONBoxController) checkHBStatus(Isd int, As int) {
	time.Sleep(HeartBeatPeriod * time.Hour)
	for true {
		slas, err := models.FindSCIONLabASByIAInt(Isd, As)
		if err != nil {
			if err == orm.ErrNoRows {
				return
			} else {
				continue
			}
		}
		delta := time.Now().Sub(slas.Updated)
		if delta.Hours() > float64(10*HeartBeatPeriod) {
			if slas.Status != models.INACTIVE {
				log.Printf("AS Status set to inactive, AS: %v, Time since last HB: %v", slas, delta)
				slas.Status = models.INACTIVE
				slas.Update()
			}

		}
		time.Sleep(HeartBeatPeriod * time.Hour)
	}
}

func userPackagePath(email string) string {
	return filepath.Join(BoxPackagePath, email)
}

func getCoreCertPath(isdcredentialpath string, isd string) string {
	return filepath.Join(isdcredentialpath, "ISD"+isd+"-AS1-V0.crt")
}

func getCoreSigKeyPath(isdcredentialpath string) string {
	return filepath.Join(isdcredentialpath, "as-sig.key")
}

func getCoreTrcFilePath(isdcredentialpath string, isd string) string {
	return filepath.Join(isdcredentialpath, "ISD"+isd+"-V0.trc")
}

func ISDCoreCredentialsPath(isd string) string {
	return filepath.Join(credentialsPath, "ISD"+isd)
}

func GetLinktype(Linktype uint8) string {
	if Linktype == models.PARENT {
		return "PARENT"
	}
	if Linktype == models.CHILD {
		return "CHILD"
	}
	return "CORE"
}
