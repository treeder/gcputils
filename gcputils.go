package gcputils

import (
	"encoding/base64"
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
	return e
}

// CredentialsOptionsFromEnv this will check an environment var call GOOGLE_ACCOUNT which can contain
// your JSON credentials base64 encoded.
// Run `base64 -w 0 account.json` to create this value.
// This will not error if it doesn't exist, so you can use this locally and let Google
// automatically get credentials when running on GCP.
func CredentialsOptionsFromEnv() ([]option.ClientOption, error) {
	opts := []option.ClientOption{}
	serviceAccountEncoded := os.Getenv("GOOGLE_ACCOUNT") // base64 encoded json creds
	if serviceAccountEncoded != "" {
		serviceAccountJSON, err := base64.StdEncoding.DecodeString(serviceAccountEncoded)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithCredentialsJSON(serviceAccountJSON))
	}
	return opts, nil
}
