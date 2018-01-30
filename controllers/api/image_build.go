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

    "github.com/netsec-ethz/scion-coord/config"
    "github.com/netsec-ethz/scion-coord/controllers"
    "github.com/netsec-ethz/scion-coord/controllers/middleware"
    "github.com/netsec-ethz/scion-coord/models"
)

// Image state
const (
    BUILDING = iota // 0
    READY
)

type customImage struct {
    Device string
    JobId string
    Status int
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
    u.userJobLock.Lock()
    defer u.userJobLock.Unlock()

    duration := time.Since(u.LastBuildRequest)
    return duration.Minutes()>float64(config.IMG_BUILD_BUILD_DELAY)
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
    imageName := r.Form["image_name"][0]

    log.Printf("Got request to build image: %s", imageName)

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

    url:=config.IMG_BUILD_ADDRESS+"/create/"+imageName
    req, err := http.NewRequest(http.MethodPost, url , body)
    req.Header.Add("Content-Type", writer.FormDataContentType())
    if err != nil {
        log.Printf("Error creating request: %v", err)
        s.Error500(w, err, "Error creating request")
        return
    }

    client := &http.Client{}
    resp, err := client.Do(req)
    
    if err != nil {
        log.Printf("Error sending request: %v", err)
        s.Error500(w, err, "Error sending request")
        return
    } else {
        var bodyContent []byte
        fmt.Println(resp.StatusCode)
        fmt.Println(resp.Header)
        resp.Body.Read(bodyContent)
        resp.Body.Close()
        log.Println(bodyContent)
    }

    fmt.Fprintln(w, "Done")
}

