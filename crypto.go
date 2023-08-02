package proton_api_bridge

import (
	"encoding/base64"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

func generatePassphrase() (string, error) {
	token, err := crypto.RandomToken(32)
	if err != nil {
		return "", err
	}

	tokenBase64 := base64.StdEncoding.EncodeToString(token)
	return tokenBase64, nil
}

func generateCryptoKey() (string, string, error) {
	passphrase, err := generatePassphrase()
	if err != nil {
		return "", "", err
	}

	// all hardcoded values from iOS drive
	key, err := helper.GenerateKey("Drive key", "noreply@protonmail.com", []byte(passphrase), "x25519", 0)
	if err != nil {
		return "", "", err
	}

	return passphrase, key, nil
}

// taken from Proton Go API Backend
func encryptWithSignature(kr, addrKR *crypto.KeyRing, b []byte) (string, string, error) {
	enc, err := kr.Encrypt(crypto.NewPlainMessage(b), nil)
	if err != nil {
		return "", "", err
	}

	encArm, err := enc.GetArmored()
	if err != nil {
		return "", "", err
	}

	sig, err := addrKR.SignDetached(crypto.NewPlainMessage(b))
	if err != nil {
		return "", "", err
	}

	sigArm, err := sig.GetArmored()
	if err != nil {
		return "", "", err
	}

	return encArm, sigArm, nil
}

func generateNodeKeys(kr, addrKR *crypto.KeyRing) (string, string, string, error) {
	nodePassphrase, nodeKey, err := generateCryptoKey()
	if err != nil {
		return "", "", "", err
	}

	nodePassphraseEnc, nodePassphraseSignature, err := encryptWithSignature(kr, addrKR, []byte(nodePassphrase))
	if err != nil {
		return "", "", "", err
	}

	return nodeKey, nodePassphraseEnc, nodePassphraseSignature, nil
}

func reencryptKeyPacket(srcKR, dstKR, addrKR *crypto.KeyRing, passphrase string) (string, error) {
	oldSplitMessage, err := crypto.NewPGPSplitMessageFromArmored(passphrase)
	if err != nil {
		return "", err
	}

	sessionKey, err := srcKR.DecryptSessionKey(oldSplitMessage.KeyPacket)
	if err != nil {
		return "", err
	}

	newKeyPacket, err := dstKR.EncryptSessionKey(sessionKey)
	if err != nil {
		return "", err
	}

	newSplitMessage := crypto.NewPGPSplitMessage(newKeyPacket, oldSplitMessage.DataPacket)

	return newSplitMessage.GetArmored()
}

func getKeyRing(kr, addrKR *crypto.KeyRing, key, passphrase, passphraseSignature string, skipSignatureVerifications bool) (*crypto.KeyRing, error) {
	enc, err := crypto.NewPGPMessageFromArmored(passphrase)
	if err != nil {
		return nil, err
	}

	dec, err := kr.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	sig, err := crypto.NewPGPSignatureFromArmored(passphraseSignature)
	if err != nil {
		return nil, err
	}

	if !skipSignatureVerifications {
		if err := addrKR.VerifyDetached(dec, sig, crypto.GetUnixTime()); err != nil {
			return nil, err
		}
	}

	lockedKey, err := crypto.NewKeyFromArmored(key)
	if err != nil {
		return nil, err
	}

	unlockedKey, err := lockedKey.Unlock(dec.GetBinary())
	if err != nil {
		return nil, err
	}

	return crypto.NewKeyRing(unlockedKey)
}
