package proton_api_bridge

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func (protonDrive *ProtonDrive) getNodeKRByID(ctx context.Context, linkID string) (*crypto.KeyRing, error) {
	if linkID == "" {
		// most likely someone requested parent link, which happen to be ""
		return protonDrive.MainShareKR.Copy() // we need to return a deep copy since the keyring will be freed by the caller when it finishes using the keyring
	}

	link, err := protonDrive.getLink(ctx, linkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.getNodeKR(ctx, link)
}
