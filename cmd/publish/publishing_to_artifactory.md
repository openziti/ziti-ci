# Publishing to Artifactory

## Setup

In order to use the publish and publish artifactory subcommands you'll need an access token from artifactory. 
As of May-20-2020 this can only be obtained via using a curl statement as the UI will grant you 'admin' 
level access and the token will expire in 3600 minutes (seconds?). 

In order to restrict the access token use the following curl statement. The username does NOT need to exist
before making the token. The `member-of-groups` MUST exist and you must not typo the group. If you find that
you're getting 401 errors - you probably typo'ed the group (like i did) or you setup the group incorrectly.

    curl -H "X-JFrog-Art-Api:$JFROG_API_KEY" \
        -X POST https://netfoundry.jfrog.io/artifactory/api/security/token \
        -d "username=ziti-tunnel-win-publish" \
        -d "scope=member-of-groups:ziti-publish-group" \
        -d "expires_in=0"

This will return a block of json similar to the following:    

    {
      "scope" : "member-of-groups:ziti-publish-group api:*",
      "access_token" : "_SET_THIS_INTO: JFROG_ACCESS_TOKEN",
      "expires_in" : 0,
      "token_type" : "Bearer"
    }

The access_token is critical - it is what will allow the artifact to be published.

A pom must also be supplied to the repository. The artifactory sub command will generate a very simple pom and publish
it along with the target artifact.

## Usage/Example Command

Once built you can deploy an artifact using the provided flags. A simple example to deploy an dummy text file looks
like this:
 
    cls && @echo "this is text" >> test.txt 
    ziti-ci publish artifactory --groupId groupid --artifactId artifactid --version 0.0.1-SNAPSHOT --target test.txt

Assuming success the output should look something like:

     ALL CHECKS PASSED
    ======================== DEPLOY BEGINS ========================
      - Url           :  https://netfoundry.jfrog.io/artifactory
      - Repository    :  ziti-maven-snapshot
      - GroupId       :  groupid
      - ArtifactId    :  artifactid
      - Version       :  0.0.1-SNAPSHOT
      - Ext           :  .txt
      - Classifier    :
      - Target        :  test.txt
    ========================  DEPLOY ENDS  ========================
    URL: test.txt
    [Info] [Thread 0] Uploading artifact: test.txt
    [Debug] [Thread 0]  Artifactory response: 201 Created
    [Debug] Uploaded 1 artifacts.
    artifact uploaded successfully to :  https://netfoundry.jfrog.io/artifactory/ziti-maven-snapshot/groupid/artifactid/0.0.1-SNAPSHOT/artifactid-0.0.1-SNAPSHOT.txt
    URL: C:\Users\cd\AppData\Local\Temp\ziti-ci-pom793211455
    [Info] [Thread 2] Uploading artifact: C:\Users\cd\AppData\Local\Temp\ziti-ci-pom793211455
    [Debug] [Thread 2]  Artifactory response: 201 Created
    [Debug] Uploaded 1 artifacts.
    artifact uploaded successfully to :  https://netfoundry.jfrog.io/artifactory/ziti-maven-snapshot/groupid/artifactid/0.0.1-SNAPSHOT/artifactid-0.0.1-SNAPSHOT.pom

When deploying a non-snapshot more than once you should get a failure indicating the artifact could not be uploaded.
A `403 Forbidden` means you are not permitted to upload that artifact again.

    ziti-ci publish artifactory --groupId groupid --artifactId artifactid --version 0.0.1 --target test.txt
    
    ALL CHECKS PASSED
    ======================== DEPLOY BEGINS ========================
      - Url           :  https://netfoundry.jfrog.io/artifactory
      - Repository    :  ziti-maven-snapshot
      - GroupId       :  groupid
      - ArtifactId    :  artifactid
      - Version       :  0.0.1
      - Ext           :  .txt
      - Classifier    :
      - UploadTarget  :  test.txt
    ========================  DEPLOY ENDS  ========================
    [Info] [Thread 0] Uploading artifact: test.txt
    [Error] [Thread 0] Artifactory response: 403 Forbidden
    
    [Error] Failed uploading 1 artifacts.
    could not publish artifact. There should be an ERROR above with a clue as to why.