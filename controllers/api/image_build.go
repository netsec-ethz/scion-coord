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
    activeJobs map[uint64]*userJobs
}

func CreateSCIONImgBuildController()(*SCIONImgBuildController){
    return &SCIONImgBuildController{
        jobsLock:&sync.Mutex{},
        activeJobs:make(map[uint64]*userJobs),
    }
}

func (s *SCIONImgBuildController) getUserBuildJobs(userId uint64)(*userJobs){
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
    return false    //TODO: REMOVE! Just for testing

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

// TODO: Refactor!
func (s *SCIONImgBuildController) GenerateImage(w http.ResponseWriter, r *http.Request) {
    log.Printf("Got request to generate image!")

    _, userSession, err := middleware.GetUserSession(r)
    if err != nil {
        log.Printf("Error getting the user session: %v", err)
        s.Forbidden(w, err, "Error getting the user session")
        return
    }
    vm, err := models.FindSCIONLabVMByUserEmail(userSession.Email)
    if err != nil || vm.Status == INACTIVE || vm.Status == REMOVE {
        log.Printf("No active configuration found for user %v\n", userSession.Email)
        s.BadRequest(w, nil, "No active configuration found for user %v",
            userSession.Email)
        return
    }

    buildJobs := s.getUserBuildJobs(userSession.UserId)
    if buildJobs.isRateLimited() {
        s.BadRequest(w, fmt.Errorf("Rate limited request"), "You have exceeded all build jobs, please wait and try again later")
        return
    }

    if err := r.ParseForm(); err != nil {
        s.BadRequest(w, fmt.Errorf("Error parsing the form: %v", err), "Error parsing form")
        return
    }

    var bRequest buildRequest
    decoder := json.NewDecoder(r.Body)

    // check if the parsing succeeded
    if err := decoder.Decode(&bRequest); err != nil {
        log.Println(err)
        s.Error500(w, err, "Error decoding JSON")
        return
    }

    log.Printf("Got request to build image: %s", bRequest.ImageName)

    fileName := userSession.Email + ".tar.gz"
    filePath := filepath.Join(PackagePath, fileName)
    data, err := ioutil.ReadFile(filePath)
    if err != nil {
        log.Printf("Error reading the tarball. FileName: %v, %v", fileName, err)
        s.Error500(w, err, "Error reading tarball")
        return
    }

    body := new(bytes.Buffer)
    writer := multipart.NewWriter(body)
    part, err := writer.CreateFormFile("config_file", fileName)
    if err != nil {
        log.Printf("Error creating form file, %v", err)
        s.Error500(w, err, "Error creating form file")
        return
    }
    part.Write(data)

    writer.WriteField("token", config.IMG_BUILD_SECRET_TOKEN)
    writer.Close()

    url:=config.IMG_BUILD_ADDRESS+"/create/"+bRequest.ImageName
    req, err := http.NewRequest(http.MethodPost, url , body)
    req.Header.Add("Content-Type", writer.FormDataContentType())
    if err != nil {
        log.Printf("Error creating request: %v", err)
        s.Error500(w, err, "Error creating request")
        return
    }

    resp, err := httpClient.Do(req)
    
    if err != nil {
        // Error in communication with server
        log.Printf("Error sending request: %v", err)
        s.Error500(w, err, "Error sending request")
        return
    } else if (resp.StatusCode!=200){
        // Proxy the server error
        w.WriteHeader(resp.StatusCode)
        io.Copy(w, resp.Body)
        return
    }

    responseBody, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        s.Error500(w, err, "Error reading response")
        return
    }

    newImage := &customImage{}
    err = json.Unmarshal(responseBody, newImage)
    if(err != nil){
        s.Error500(w, err, "Error parsing request from server")
        return
    }

    newImage.Status=BUILDING
    buildJobs.addImage(newImage)

    fmt.Fprintln(w, "Done")   
}

func (s *SCIONImgBuildController) GetUserImages(w http.ResponseWriter, r *http.Request) {
    log.Printf("Got request to generate image!")

    _, userSession, err := middleware.GetUserSession(r)
    if err != nil {
        log.Printf("Error getting the user session: %v", err)
        s.Forbidden(w, err, "Error getting the user session")
        return
    }
    
    buildJobs := s.getUserBuildJobs(userSession.UserId)
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