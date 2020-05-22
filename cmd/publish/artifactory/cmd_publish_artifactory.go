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
"github.com/spf13/cobra"
)

var m = MavenLayout{}

func NewPublishArtifactoryCmd() *cobra.Command {

	command := &cobra.Command{
		Use:   "artifactory",
		Short: "Publishes an artifact to an artifactory",
		Long:  "Expects a maven-style repository to be specified. Defaults to " + DefaultRepo + " if not specified. " +
			"Supports -SNAPSHOT versions to be specified",
		Run: func(cmd *cobra.Command, args []string) {
			m.Publish()
		},
	}

	command.Flags().StringVar(&m.Repository, "repository", "", "the repo to publish to")
	command.Flags().StringVar(&m.GroupId, "groupId", "", "the groupId to use. example: netfoundry.ziti")
	command.Flags().StringVar(&m.ArtifactId, "artifactId", "", "the artifactId to use. example: ziti-ci")
	command.Flags().StringVar(&m.Version, "version", "", "the version to deploy")
	command.Flags().StringVar(&m.UploadTarget, "target", "", "the file to upload/publish")
	return command
}
