/*
Copyright © 2019 Nuxeo

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
  "fmt"
  "os"
  "os/user"
  "github.com/spf13/cobra"
  "github.com/sirupsen/logrus"
  "github.com/nuxeo-cloud/aws-iam-credential-rotate/lib"
)


var cfgFile string

var log = logrus.New()

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
  Use:   "rotate-iam",
  Short: "A utility that rotates IAM credientials contained in a k8s secret",
  Long: `A utility that rotates IAM credientials contained in a k8s secret.`,
  // Uncomment the following line if your bare application
  // has an action associated with it:
  //	Run: func(cmd *cobra.Command, args []string) { },
}

// volumeCmd represents the volume command
var rotateCmd = &cobra.Command{
  Use:   "rotate",
  Short: "Rotate the keys labelized.",
  Long: `Rotates the IAM key`,
  Run: func(cmd *cobra.Command, args []string) {

    usr, err := user.Current()
    if err != nil {
        log.Fatal( err )
    }


    kubeConfigPath := usr.HomeDir + "/.kube/config"
    fmt.Println("looking for " + kubeConfigPath)
    if _, err := os.Stat(kubeConfigPath); os.IsNotExist(err) {
      kubeConfigPath = ""
      log.Info("Using in-cluster configuration")
    } else {
      log.Info("Reading k8s configuration from: " + kubeConfigPath)
    }


    client, err := lib.LoadClient(kubeConfigPath)
    if err != nil {
        log.Fatal( err )
    }

    namespace, exists := os.LookupEnv("NAMESPACE")
    if(!exists) {
      namespace = client.Namespace
    }



    lib.RotateKeys(client, namespace)

  },
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
  if err := rootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}

func init() {
  cobra.OnInitialize(initConfig)
  rootCmd.AddCommand(rotateCmd)
}


// initConfig reads in config file and ENV variables if set.
func initConfig() {


}

