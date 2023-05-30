package gcputils

import (
	"context"
	"fmt"
	"strings"

	"github.com/treeder/gotils/v2"
	kmspb "google.golang.org/genproto/googleapis/cloud/kms/v1"
	kms "cloud.google.com/go/kms/apiv1"
)

// Encrypt using Google KMS to encrypt data
// keyName is just the last part of the default keyring we're using
func Encrypt(ctx context.Context, kmsClient *kms.KeyManagementClient, projectID, region, keyRingName, keyName string, data []byte) ([]byte, error) {
	kmsKeyName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, region, keyRingName, keyName)
	req := &kmspb.EncryptRequest{
		Name:      kmsKeyName,
		Plaintext: data,
	}
	var ciphertext []byte
	for i := 0; ; i++ {
		resp, err := kmsClient.Encrypt(ctx, req)
		if err != nil {
			gotils.L(ctx).Error().Println("error during kmsclient.encrypt", (err))
			if i < 2 && strings.Contains(err.Error(), "transport is closing") {
				gotils.L(ctx).Error().Println("SHOULD try TO reopen KMS connection and try again if we see this")
				continue
			}
			return nil, err
		}
		ciphertext = resp.Ciphertext
		break
	}
	return ciphertext, nil
}

// Decrypt using Google KMS to encrypt data
// keyName is just the last part of the default keyring we're using
func Decrypt(ctx context.Context, kmsClient *kms.KeyManagementClient,  projectID, region, keyRingName,keyName string, data []byte) ([]byte, error) {
	kmsKeyName := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s",
		projectID, region, keyRingName, keyName)
	req := &kmspb.DecryptRequest{
		Name:       kmsKeyName,
		Ciphertext: data,
	}
	var plaintext []byte
	for i := 0; ; i++ {
		resp, err := kmsClient.Decrypt(ctx, req)
		if err != nil {
			gotils.L(ctx).Error().Println("error during kmsclient.decrypt", (err))
			if i < 2 && strings.Contains(err.Error(), "transport is closing") {
				gotils.L(ctx).Error().Println("SHOULD try TO reopen KMS connection and try again if we see this")
				continue
			}
			return nil, err
		}
		plaintext = resp.Plaintext
		break
	}
	return plaintext, nil
}
