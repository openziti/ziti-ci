/*
 * Copyright NetFoundry, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package artifactory

import (
  "bufio"
  "fmt"
  "github.com/jfrog/jfrog-client-go/artifactory"
  "github.com/jfrog/jfrog-client-go/artifactory/auth"
  "github.com/jfrog/jfrog-client-go/artifactory/services"
  "github.com/jfrog/jfrog-client-go/config"
  "github.com/jfrog/jfrog-client-go/utils/log"
  "io/ioutil"
  "os"
  "strings"
  "text/template"
)

const DefaultUrl = "https://netfoundry.jfrog.io/artifactory"
const DefaultRepo = "ziti-maven-snapshot"

var file *os.File

type MavenLayout struct {
  Url        string
  Repository string
  GroupId    string
  ArtifactId string
  Version    string
  Ext        string
  Classifier   string
  UploadTarget string

  accessToken string
}

func(m *MavenLayout) Deploy() {
  checkInput(m)

  fmt.Println()
  fmt.Println("ALL CHECKS PASSED")
  fmt.Println("======================== DEPLOY BEGINS ========================")
  fmt.Println("  - Url           : ", m.Url)
  fmt.Println("  - Repository    : ", m.Repository)
  fmt.Println("  - GroupId       : ", m.GroupId)
  fmt.Println("  - ArtifactId    : ", m.ArtifactId)
  fmt.Println("  - Version       : ", m.Version)
  fmt.Println("  - Ext           : ", m.Ext)
  fmt.Println("  - Classifier    : ", m.Classifier)
  fmt.Println("  - UploadTarget  : ", m.UploadTarget)
  fmt.Println("========================  DEPLOY ENDS  ========================")

  if strings.TrimSpace(m.Url) == "" {
    fmt.Printf("using default url: %s\n", DefaultUrl)
    m.Url = DefaultUrl
  }
  if strings.TrimSpace(m.Repository) == "" {
    m.Repository = DefaultRepo
  }

  artifactoryDetails := auth.NewArtifactoryDetails()
  artifactoryDetails.SetUrl(m.Url + "/" + m.Repository)
  artifactoryDetails.SetAccessToken(m.accessToken)

  serviceConfig, err := config.NewConfigBuilder().SetServiceDetails(artifactoryDetails).Build()

  if err != nil {
    fmt.Println("unexpected error :", err)
    os.Exit(3)
  }

  rtManager, err := artifactory.New(&artifactoryDetails, serviceConfig)

  if err != nil {
    fmt.Println("unexpected error :", err)
    os.Exit(3)
  }

  l, err := m.layout()
  if err != nil {
   fmt.Println("unexpected error :", err)
   os.Exit(3)
  }

  params := services.NewUploadParams()
  params.Target = l
  params.Pattern = m.UploadTarget

  log.SetLogger(log.NewLogger(log.INFO, os.Stdout))

  artifactsFileInfo, totalUploaded, totalFailed, err := rtManager.UploadFiles(params)

  if err != nil {
    fmt.Println("unexpected error :", err, artifactsFileInfo, totalUploaded, totalFailed)
    os.Exit(3)
  }

  if totalFailed > 0 {
    fmt.Println("could not publish artifact. There should be an ERROR above with a clue as to why.")
    os.Exit(3)
  }

  for _, art := range artifactsFileInfo {
    fmt.Println("artifact uploaded successfully to : ", art.ArtifactoryPath)
  }

  //create and emit a temporary file for the pom - without the pom artifactory won't allow the same file to be uploaded again
  pom, err := ioutil.TempFile(os.TempDir(), "ziti-ci-pom")
  if err != nil {
    fmt.Println("unexpected error creating pom file:", err)
    os.Exit(3)
  }
  defer os.Remove(pom.Name()) // clean up

  w := bufio.NewWriter(pom)

  pomTemplate := `<?xml version="1.0" encoding="UTF-8"?>
<project 
  xmlns="http://maven.apache.org/POM/4.0.0" 
  xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" 
  xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
  http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <groupId>{{.GroupId}}</groupId>
  <artifactId>{{.ArtifactId}}</artifactId>
  <version>{{.Version}}</version>
  <description>ziti-ci auto generated POM</description>
  <type>{{.Ext}}</type>
</project>`

  t := template.Must(template.New("pomTemplate").Parse(pomTemplate))
  _ = t.Execute(w, m)
  _ = w.Flush()
  _ = pom.Close()

  m.Ext = ".pom"
  l, err = m.layout()
  if err != nil {
    fmt.Println("unexpected error :", err)
    os.Exit(3)
  }
  params = services.NewUploadParams()
  params.Target = l
  params.Pattern = pom.Name()

  log.SetLogger(log.NewLogger(log.DEBUG, os.Stdout))

  artifactsFileInfo, totalUploaded, totalFailed, err = rtManager.UploadFiles(params)

  if err != nil {
    fmt.Println("unexpected error :", err, artifactsFileInfo, totalUploaded, totalFailed)
    os.Exit(3)
  }

  for _, art := range artifactsFileInfo {
    fmt.Println("artifact uploaded successfully to : ", art.ArtifactoryPath)
  }
}

func checkInput(m *MavenLayout) {
  errd := false

  if strings.TrimSpace(m.GroupId) == "" {
    errd = true
    fmt.Println("GroupId is required and was not supplied")
  }

  if strings.TrimSpace(m.ArtifactId) == "" {
    errd = true
    fmt.Println("ArtifactId is required and was not supplied")
  }

  if strings.TrimSpace(m.Version) == "" {
    errd = true
    fmt.Println("Version is required and was not supplied")
  }

  if strings.TrimSpace(m.Ext) == "" {
    // use the extension from the target
    if strings.TrimSpace(m.UploadTarget) != "" {
      li := strings.LastIndex(m.UploadTarget, ".")
      if li > 0 {
        m.Ext = m.UploadTarget[li:]
      } else {
        errd = true
        fmt.Println("Ext is required and was not supplied and an extension could not be pulled from the Target [%v]", m.UploadTarget)
      }
    } else {
      errd = true
      fmt.Println("Ext is required and was not supplied")
    }
  }

  if strings.TrimSpace(m.UploadTarget) == "" {
    errd = true
    fmt.Println("Target is required and was not supplied")
  }

  m.accessToken = os.Getenv("JFROG_ACCESS_TOKEN")

  if strings.TrimSpace(m.accessToken) == "" {
    errd = true
    fmt.Println("JFROG_ACCESS_TOKEN is required and was not found")
  }

  if strings.TrimSpace(m.Url) == "" {
    m.Url = DefaultUrl
  }

  if strings.TrimSpace(m.Repository) == "" {
    m.Repository = DefaultRepo
  }

  if errd {
    os.Exit(3)
  }
}

func(m *MavenLayout) layout() (string, error){
  /*
    maven format:
    [url]/[repo]/[groupId]/[artifactId]/[version]/[artifact]-[version](-[classifier])(.[ext])

    real life example:
    https://repo1.maven.org/maven2/org/apache/cxf/cxf-rt-transports-http-jetty/3.3.6/cxf-rt-transports-http-jetty-3.3.6.jar

    <groupId>junit</groupId>
    <artifactId>junit</artifactId>
    <version>4.12</version>
    <type>jar</type>
    <scope>test</scope>
  */

  org := strings.ReplaceAll(m.GroupId, ".", "/")
  l := fmt.Sprintf("/%s/%s/%s/%s-%s", org, m.ArtifactId, m.Version, m.ArtifactId, m.Version)
  if m.Classifier != "" {
    l = l + "-" + m.Classifier
  }
  l = l + m.Ext

  return l, nil
}