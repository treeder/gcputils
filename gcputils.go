package gcputils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/compute/metadata"
	"google.golang.org/api/option"
)

// GetEnvVar def is default, leave blank to fatal if not found
// checks in env, then GCE metadata
func GetEnvVar(name, def string) string {
	var err error
	e := os.Getenv(name)
	if e != "" {
		return e
	}
	// check if a metadata.json file exists, this is the file downloaded from google "REST equivalent" in metadata section
	// if len(metaFileItems) > 0 {
	// 	// see if we loaded it from a file
	// 	for _, kv := range metaFileItems {
	// 		fmt.Println(kv.Key, kv.Value)
	// 		if kv.Key == name {
	// 			return kv.Value
	// 		}
	// 	}
	// }
	if metadata.OnGCE() {
		e, err = metadata.ProjectAttributeValue(name)
		if err == nil {
			fmt.Println("GOT META", e)
			return e
		}
		log.Println("error on metadata.ProjectAttributeValue", err)
	}
	if def != "" {
		return def
	}
	// log.Info().Str(name, tgApiKey).Msg("Got from metadata server")
	log.Fatalf("NO %v", name)
	// todo: this shouldn't fatal like this, kind of annoying
	return e
}

// ProjectID returns the project ID by looking in several places in the following order of preference:
// * env var passed in via envVarName
// * set in GCE metadata with matching name to envVarName (user defined)
// * instance metadata
// * in credentials key/json
//
// gKeyEnvVarName is required only if not running on GCP compute
// projectIDEnvVarName is optional
func CredentialsAndProjectIDFromEnv(gKeyEnvVarName, projectIDEnvVarName string) ([]option.ClientOption, string, error) {
	gj, opts, err := KeyAndOptionsFromEnv(gKeyEnvVarName)
	if err != nil {
		return nil, "", err
	}
	gProjectID := GetEnvVar(projectIDEnvVarName, "x")
	if gProjectID != "x" && gProjectID != "" {
		return opts, gProjectID, nil
	}
	if metadata.OnGCE() {
		gProjectID2, err := metadata.ProjectID()
		if err != nil {
			fmt.Println("gprojectID2 error:", err)
		}
		fmt.Println("PROJECT_ID FROM METADATA: ", gProjectID2)
		return opts, gProjectID2, nil
	}
	// and lastly from JSON
	return opts, gj.ProjectID, nil
}

// CredentialsOptionsFromEnv this will check an environment var with key you provide, which should contain
// your JSON credentials base64 encoded. Can passed returned value directly into clients.
// Run `base64 -w 0 account.json` to create this value.
// This also supports running on GCP, just don't set this environment variable or metadata on GCP.
// This will not error if it doesn't exist, so you can use this locally and let Google
// automatically get credentials when running on GCP.
func CredentialsOptionsFromEnv(envKey string) ([]option.ClientOption, error) {
	opts := []option.ClientOption{}
	serviceAccountEncoded := GetEnvVar(envKey, "x") // base64 encoded json creds
	if serviceAccountEncoded == "x" {
		return opts, nil
	}
	serviceAccountJSON, err := base64.StdEncoding.DecodeString(serviceAccountEncoded)
	if err != nil {
		return nil, err
	}
	opts = append(opts, option.WithCredentialsJSON(serviceAccountJSON))
	return opts, nil
}

// CredentialsAndOptionsFromEnv this will check an environment var with key you provide, which should contain
// your JSON credentials base64 encoded. Can passed returned value directly into clients.
// Run `base64 -w 0 account.json` to create this value.
// This also supports running on GCP, just don't set this environment variable or metadata on GCP.
// This will not error if it doesn't exist, so you can use this locally and let Google
// automatically get credentials when running on GCP.
func KeyAndOptionsFromEnv(envKey string) (*GoogleJSON, []option.ClientOption, error) {
	opts := []option.ClientOption{}
	serviceAccountEncoded := GetEnvVar(envKey, "x") // base64 encoded json creds
	if serviceAccountEncoded == "x" {
		return nil, opts, nil
	}
	serviceAccountJSON, err := base64.StdEncoding.DecodeString(serviceAccountEncoded)
	if err != nil {
		return nil, nil, err
	}
	opts = append(opts, option.WithCredentialsJSON(serviceAccountJSON))
	gj := &GoogleJSON{}
	err = json.Unmarshal(serviceAccountJSON, gj)
	if err != nil {
		return nil, nil, err
	}
	return gj, opts, nil
}

// GoogleJSON is the struct you get when you create a new service account
type GoogleJSON struct {
	ProjectID string `json:"project_id"`
}
