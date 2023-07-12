package proton_api_bridge

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
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

func (protonDrive *ProtonDrive) getNodeKR(ctx context.Context, link *proton.Link) (*crypto.KeyRing, error) {
	if link.ParentLinkID == "" {
		nodeKR, err := link.GetKeyRing(protonDrive.MainShareKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}

		return nodeKR, nil
	}

	parentLink, err := protonDrive.getLink(ctx, link.ParentLinkID)
	if err != nil {
		return nil, err
	}

	// parentNodeKR is used to decrypt the current node's KR, as each node has its keyring, which can be decrypted by its parent
	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return nil, err
	}

	nodeKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}

	return nodeKR, nil
}
