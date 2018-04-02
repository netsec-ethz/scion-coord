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
    "path/filepath"
    "io"
    "time"
    "mime/multipart"
    "sync"
    "encoding/json"

    "github.com/gorilla/mux"

    "github.com/netsec-ethz/scion-coord/config"
    "github.com/netsec-ethz/scion-coord/controllers"
    "github.com/netsec-ethz/scion-coord/controllers/middleware"
    "github.com/netsec-ethz/scion-coord/models"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Image state
const (
    BUILDING = iota // 0
    READY
)

type buildRequest struct {
    ImageName string `json:"image_name"`
}

type jobStatus struct {
    Exists bool   `json:"job_exists"`
    Finished bool   `json:"build_finished"`
    JobId string    `json:"job_id"`
}

type customImage struct {
    Device string   `json:"image"`
    JobId string    `json:"id"`
    Status int
}

func (c *customImage) MarshalJSON() ([]byte, error) {  
    m := make(map[string]string)
    m["image"] = c.Device
    m["id"] = c.JobId
    m["download_link"] = config.IMG_BUILD_ADDRESS+"/download/"+c.JobId
    if(c.Status==READY){
        m["status"]="ready"
    }else{
        m["status"]="building"
    }
    return json.Marshal(m)
}

type userJobs struct {
    userJobLock *sync.Mutex

    LastBuildRequest time.Time
    UserImages []*customImage
}

type SCIONImgBuildController struct {
    controllers.HTTPController

    // Keep track of users jobs
    jobsLock *sync.Mutex
    activeJobs map[string]*userJobs //FIXME! This should be integer not string, userIds are not unique check why?
}

func CreateSCIONImgBuildController()(*SCIONImgBuildController){
    return &SCIONImgBuildController{
        jobsLock:&sync.Mutex{},
        activeJobs:make(map[string]*userJobs),
    }
}

func (s *SCIONImgBuildController) getUserBuildJobs(userId string)(*userJobs){
    s.jobsLock.Lock()
    defer s.jobsLock.Unlock()

    if userJob, ok := s.activeJobs[userId]; ok{
        return userJob;
    }else{
        newUserJob:=&userJobs{
            LastBuildRequest:time.Unix(0,0),    // last build time - beginning of time
            userJobLock:&sync.Mutex{},
            UserImages:make([]*customImage, 0),
        }

        s.activeJobs[userId]=newUserJob

        return newUserJob
    }
}

func (u *userJobs) isRateLimited()(bool){
    u.userJobLock.Lock()
    defer u.userJobLock.Unlock()

    duration := time.Since(u.LastBuildRequest)
    return duration.Minutes()<float64(config.IMG_BUILD_BUILD_DELAY)
}

func (u *userJobs) addImage(image *customImage){
    u.userJobLock.Lock()
    defer u.userJobLock.Unlock()

    u.LastBuildRequest=time.Now()
    u.UserImages=append(u.UserImages, image)
}

// Returns copy of slice so we don't run in concurrency issues
func (u *userJobs) getUserImages()([]*customImage){
    u.userJobLock.Lock()
    defer u.userJobLock.Unlock()

    result := make([]*customImage, len(u.UserImages))
    copy(result, u.UserImages)

    return result
}

func (u *userJobs) removeImage(jobId string){
    u.userJobLock.Lock()
    defer u.userJobLock.Unlock()

    for i, img := range u.UserImages{
        if(img.JobId==jobId){

            u.UserImages[len(u.UserImages)-1], u.UserImages[i] = 
                u.UserImages[i], u.UserImages[len(u.UserImages)-1]
            u.UserImages=u.UserImages[:len(u.UserImages)-1]

            return
        }
    }
}

func (s *SCIONImgBuildController) GetAvailableDevices(w http.ResponseWriter, r *http.Request) {
    resp, err := http.Get(config.IMG_BUILD_ADDRESS+"/get-images")
    if err != nil {
        s.Error500(w, err, "Error contacting remote image building service!")
        return
    }
    defer resp.Body.Close()

    w.Header().Set("Content-Type", "application/json")
    io.Copy(w, resp.Body)
}

func (s *SCIONImgBuildController) GenerateImage(w http.ResponseWriter, r *http.Request) {
    log.Printf("Got request to generate image!")

    // Get user session
    _, uSess, err := middleware.GetUserSession(r)
    if err != nil {
        log.Printf("Error getting the user session: %v", err)
        s.Forbidden(w, err, "Error getting the user session")
        return
    }

    // Get configuration file for specified AS
    vars := mux.Vars(r)
    asID := vars["as_id"]
    as, err := models.FindSCIONLabASByUserEmailAndASID(uSess.Email, asID)
    if err != nil || as.Status == models.INACTIVE || as.Status == models.REMOVE {
        log.Printf("No active configuration found for user %v with asId %v\n", uSess.Email, asID)
        s.BadRequest(w, nil, "No active configuration found for user %v",
            uSess.Email)
        return
    }

    if as.Type!=models.DEDICATED {
        log.Printf("Configuration for selected AS is not made for dedicated system\n")
        s.BadRequest(w, nil, "You must reconfigure your AS to use dedicated system configuration")
        return
    }

    fileName := UserPackageName(uSess.Email, as.ISD, as.ASID) + ".tar.gz"
    filePath := filepath.Join(PackagePath, fileName)

    // Get build request
    if err := r.ParseForm(); err != nil {
        s.BadRequest(w, fmt.Errorf("Error parsing the form: %v", err), "Error parsing form")
        return
    }
    var bRequest buildRequest
    decoder := json.NewDecoder(r.Body)

    if err := decoder.Decode(&bRequest); err != nil {
        log.Println(err)
        s.Error500(w, err, "Error decoding build request JSON")
        return
    }

    // Start build job build job
    buildJobs := s.getUserBuildJobs(uSess.Email)
    if buildJobs.isRateLimited() {
        s.BadRequest(w, fmt.Errorf("Rate limited request"), "You have exceeded all build jobs, please wait and try again later")
        return
    }

    if err := startBuildJob(fileName, filePath, bRequest, buildJobs); err!=nil{
        log.Println(err)
        //TODO: Reset last build job time, rate limiting should not apply here
        s.Error500(w, err, "Error running build job")
        return
    }

    message := "We started configuring image for your IoT device." +
        "Please wait few minutes for build to finish."
    fmt.Fprintln(w, message)
}

func startBuildJob(configFileName, configFilePath string, bRequest buildRequest, buildJobs *userJobs)(error){

    data, err := ioutil.ReadFile(configFilePath)
    if err != nil {
        return fmt.Errorf("Error reading configuration file [%s] %v", configFileName, err)
    }
    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)
    part, err := writer.CreateFormFile("config_file", configFileName)
    if err != nil {
        return err
    }
    part.Write(data)

    writer.WriteField("token", config.IMG_BUILD_SECRET_TOKEN)
    writer.Close()

    url:=config.IMG_BUILD_ADDRESS+"/create/"+bRequest.ImageName
    req, err := http.NewRequest(http.MethodPost, url , body)
    log.Printf("Sending request to: %s", url)
    req.Header.Add("Content-Type", writer.FormDataContentType())
    if err != nil {
        return err
    }

    resp, err := httpClient.Do(req)
    
    if err != nil {
        // Error in communication with server
        return err
    } else if (resp.StatusCode!=200){
        // Server returned an error!
        buf := new(bytes.Buffer)
        buf.ReadFrom(resp.Body)
        return fmt.Errorf("Server returned %d error: %s", resp.StatusCode, buf.String())
    }

    responseBody, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return err
    }

    newImage := &customImage{}
    err = json.Unmarshal(responseBody, newImage)
    if(err != nil){
        return err
    }

    newImage.Status=BUILDING
    buildJobs.addImage(newImage)

    return nil
}

func (s *SCIONImgBuildController) GetUserImages(w http.ResponseWriter, r *http.Request) {
    log.Printf("Requesting user images")

    _, userSession, err := middleware.GetUserSession(r)
    if err != nil {
        log.Printf("Error getting the user session: %v", err)
        s.Forbidden(w, err, "Error getting the user session")
        return
    }
    
    buildJobs := s.getUserBuildJobs(userSession.Email)
    userImages := buildJobs.getUserImages()

    for _, img := range userImages {
        exists, finished, _ := getJobStatus(img.JobId)  //TODO: Handle error
        if(finished){
            img.Status=READY
        }

        if(!exists){
            buildJobs.removeImage(img.JobId)
        }
    }
    
    userImages=buildJobs.getUserImages()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(userImages)
}

func getJson(url string, target interface{}) error {
    r, err := httpClient.Get(url)
    if err != nil {
        return err
    }
    defer r.Body.Close()

    return json.NewDecoder(r.Body).Decode(target)
}

func getJobStatus(id string)(bool, bool, error){
    status := &jobStatus{}
    err := getJson(config.IMG_BUILD_ADDRESS+"/status/"+id, status)
    if(err!=nil){
        return false, false, err
    }
    
    return status.Exists, status.Finished, nil
}