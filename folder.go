package proton_api_bridge

import (
	"context"
	"log"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type ProtonDirectoryData struct {
	Link     *proton.Link
	Name     string
	IsFolder bool
}

func (protonDrive *ProtonDrive) ListDirectory(
	ctx context.Context,
	folderLinkID string) ([]*ProtonDirectoryData, error) {
	ret := make([]*ProtonDirectoryData, 0)

	folderLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, folderLinkID)
	if err != nil {
		return nil, err
	}

	if folderLink.State == proton.LinkStateActive {
		childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
		if err != nil {
			return nil, err
		}

		if childrenLinks != nil {
			folderParentKR, err := protonDrive.getNodeKRByID(ctx, folderLink.ParentLinkID)
			if err != nil {
				return nil, err
			}
			defer folderParentKR.ClearPrivateParams()
			folderLinkKR, err := folderLink.GetKeyRing(folderParentKR, protonDrive.AddrKR)
			if err != nil {
				return nil, err
			}
			defer folderLinkKR.ClearPrivateParams()

			for i := range childrenLinks {
				if childrenLinks[i].State != proton.LinkStateActive {
					continue
				}

				name, err := childrenLinks[i].GetName(folderLinkKR, protonDrive.AddrKR)
				if err != nil {
					return nil, err
				}
				ret = append(ret, &ProtonDirectoryData{
					Link:     &childrenLinks[i],
					Name:     name,
					IsFolder: childrenLinks[i].Type == proton.LinkTypeFolder,
				})
			}
		}
	}

	return ret, nil
}

func (protonDrive *ProtonDrive) ListDirectoriesRecursively(
	ctx context.Context,
	parentNodeKR *crypto.KeyRing,
	link *proton.Link,
	download bool,
	maxDepth, curDepth /* 0-based */ int,
	excludeRoot bool,
	pathSoFar string,
	paths *[]string) error {
	/*
		Assumptions:
		- we only care about the active ones
	*/
	if link.State != proton.LinkStateActive {
		return nil
	}
	// log.Println("curDepth", curDepth, "pathSoFar", pathSoFar)

	var currentPath = ""

	if !(excludeRoot && curDepth == 0) {
		name, err := link.GetName(parentNodeKR, protonDrive.AddrKR)
		if err != nil {
			return err
		}

		currentPath = pathSoFar + "/" + name
		// log.Println("currentPath", currentPath)
		if paths != nil {
			*paths = append(*paths, currentPath)
		}
	}

	if download {
		if protonDrive.Config.DataFolderName == "" {
			return ErrDataFolderNameIsEmpty
		}

		if link.Type == proton.LinkTypeFile {
			log.Println("Downloading", currentPath)
			defer log.Println("Completes downloading", currentPath)

			byteArray, _, err := protonDrive.DownloadFile(ctx, link)
			if err != nil {
				return err
			}

			err = os.WriteFile("./"+protonDrive.Config.DataFolderName+"/"+currentPath, byteArray, 0777)
			if err != nil {
				return err
			}
		} else /* folder */ {
			if !(excludeRoot && curDepth == 0) {
				// log.Println("Creating folder", currentPath)
				// defer log.Println("Completes creating folder", currentPath)

				err := os.Mkdir("./"+protonDrive.Config.DataFolderName+"/"+currentPath, 0777)
				if err != nil {
					return err
				}
			}
		}
	}

	if maxDepth == -1 || curDepth < maxDepth {
		if link.Type == proton.LinkTypeFolder {
			childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, link.LinkID, true)
			if err != nil {
				return err
			}
			// log.Printf("childrenLinks len = %v, %#v", len(childrenLinks), childrenLinks)

			if childrenLinks != nil {
				// get current node's keyring
				linkKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
				if err != nil {
					return err
				}
				defer linkKR.ClearPrivateParams()

				for _, childLink := range childrenLinks {
					err = protonDrive.ListDirectoriesRecursively(ctx, linkKR, &childLink, download, maxDepth, curDepth+1, excludeRoot, currentPath, paths)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func (protonDrive *ProtonDrive) CreateNewFolderByID(ctx context.Context, parentLinkID string, folderName string) (string, error) {
	parentLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, parentLinkID)
	if err != nil {
		return "", err
	}

	return protonDrive.CreateNewFolder(ctx, &parentLink, folderName)
}

func (protonDrive *ProtonDrive) CreateNewFolder(ctx context.Context, parentLink *proton.Link, folderName string) (string, error) {
	// TODO: check for duplicated folder name

	parentNodeKR, err := protonDrive.getNodeKR(ctx, parentLink)
	if err != nil {
		return "", err
	}

	newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature, err := generateNodeKeys(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return "", err
	}

	createFolderReq := proton.CreateFolderReq{
		ParentLinkID: parentLink.LinkID,

		// Name string
		// Hash string

		NodeKey: newNodeKey,
		// NodeHashKey string

		NodePassphrase:          newNodePassphraseEnc,
		NodePassphraseSignature: newNodePassphraseSignature,

		SignatureAddress: protonDrive.signatureAddress,
	}

	/* Name is encrypted using the parent's keyring, and signed with address key */
	err = createFolderReq.SetName(folderName, protonDrive.AddrKR, parentNodeKR)
	if err != nil {
		return "", err
	}

	parentHashKey, err := parentLink.GetHashKey(parentNodeKR)
	if err != nil {
		return "", err
	}
	err = createFolderReq.SetHash(folderName, parentHashKey)
	if err != nil {
		return "", err
	}

	newNodeKR, err := getKeyRing(parentNodeKR, protonDrive.AddrKR, newNodeKey, newNodePassphraseEnc, newNodePassphraseSignature)
	if err != nil {
		return "", err
	}
	err = createFolderReq.SetNodeHashKey(newNodeKR)
	if err != nil {
		return "", err
	}

	createFolderResp, err := protonDrive.c.CreateFolder(ctx, protonDrive.MainShare.ShareID, createFolderReq)
	if err != nil {
		return "", err
	}

	// log.Printf("createFolderResp %#v", createFolderResp)

	return createFolderResp.ID, nil
}

func (protonDrive *ProtonDrive) MoveFolderByID(ctx context.Context, srcLinkID, destParentLinkID, destName string) error {
	srcLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, srcLinkID)
	if err != nil {
		return err
	}
	if srcLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	destParentLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, destParentLinkID)
	if err != nil {
		return err
	}
	if destParentLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	return protonDrive.MoveFolder(ctx, &srcLink, &destParentLink, destName)
}

func (protonDrive *ProtonDrive) MoveFolder(ctx context.Context, srcLink *proton.Link, destParentLink *proton.Link, destName string) error {
	// we are moving the srcLink to under destParentLink, with name destName
	req := proton.MoveLinkReq{
		ParentLinkID:     destParentLink.LinkID,
		OriginalHash:     srcLink.Hash,
		SignatureAddress: protonDrive.signatureAddress,
	}

	destParentKR, err := protonDrive.getNodeKR(ctx, destParentLink)
	if err != nil {
		return err
	}

	err = req.SetName(destName, protonDrive.AddrKR, destParentKR)
	if err != nil {
		return err
	}

	destParentHashKey, err := destParentLink.GetHashKey(destParentKR)
	if err != nil {
		return err
	}
	err = req.SetHash(destName, destParentHashKey)
	if err != nil {
		return err
	}

	srcParentKR, err := protonDrive.getNodeKRByID(ctx, srcLink.ParentLinkID)
	if err != nil {
		return err
	}
	nodePassphrase, err := reencryptKeyPacket(srcParentKR, destParentKR, protonDrive.AddrKR, srcLink.NodePassphrase)
	if err != nil {
		return err
	}
	req.NodePassphrase = nodePassphrase
	req.NodePassphraseSignature = srcLink.NodePassphraseSignature

	return protonDrive.c.MoveLink(ctx, protonDrive.MainShare.ShareID, srcLink.LinkID, req)
}
