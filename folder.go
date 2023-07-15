package proton_api_bridge

import (
	"context"
	"log"
	"os"
	"time"

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

	folderLink, err := protonDrive.getLink(ctx, folderLinkID)
	if err != nil {
		return nil, err
	}

	if folderLink.State == proton.LinkStateActive {
		childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
		if err != nil {
			return nil, err
		}

		if childrenLinks != nil {
			folderParentKR, err := protonDrive.getLinkKRByID(ctx, folderLink.ParentLinkID)
			if err != nil {
				return nil, err
			}
			folderLinkKR, err := folderLink.GetKeyRing(folderParentKR, protonDrive.AddrKR)
			if err != nil {
				return nil, err
			}

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
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(parentLinkID, false)

	parentLink, err := protonDrive.getLink(ctx, parentLinkID)
	if err != nil {
		return "", err
	}

	return protonDrive.CreateNewFolder(ctx, parentLink, folderName)
}

func (protonDrive *ProtonDrive) CreateNewFolder(ctx context.Context, parentLink *proton.Link, folderName string) (string, error) {
	parentNodeKR, err := protonDrive.getLinkKR(ctx, parentLink)
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

	// FIXME: check for duplicated filename by relying on checkAvailableHashes
	// if the folder name already exist, this call will return an error
	createFolderResp, err := protonDrive.c.CreateFolder(ctx, protonDrive.MainShare.ShareID, createFolderReq)
	if err != nil {
		return "", err
	}
	// log.Printf("createFolderResp %#v", createFolderResp)

	return createFolderResp.ID, nil
}

func (protonDrive *ProtonDrive) MoveFileByID(ctx context.Context, srcLinkID, dstParentLinkID string, dstName string) error {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(srcLinkID, false)

	srcLink, err := protonDrive.getLink(ctx, srcLinkID)
	if err != nil {
		return err
	}
	if srcLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	dstParentLink, err := protonDrive.getLink(ctx, dstParentLinkID)
	if err != nil {
		return err
	}
	if dstParentLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	return protonDrive.MoveFile(ctx, srcLink, dstParentLink, dstName)
}

func (protonDrive *ProtonDrive) MoveFile(ctx context.Context, srcLink *proton.Link, dstParentLink *proton.Link, dstName string) error {
	return protonDrive.MoveFolder(ctx, srcLink, dstParentLink, dstName)
}

func (protonDrive *ProtonDrive) MoveFolderByID(ctx context.Context, srcLinkID, dstParentLinkID, dstName string) error {
	/* It's like event system, we need to get the latest information before creating the move request! */
	protonDrive.removeLinkIDFromCache(srcLinkID, false)

	srcLink, err := protonDrive.getLink(ctx, srcLinkID)
	if err != nil {
		return err
	}
	if srcLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	dstParentLink, err := protonDrive.getLink(ctx, dstParentLinkID)
	if err != nil {
		return err
	}
	if dstParentLink.State != proton.LinkStateActive {
		return ErrLinkMustBeActive
	}

	return protonDrive.MoveFolder(ctx, srcLink, dstParentLink, dstName)
}

func (protonDrive *ProtonDrive) MoveFolder(ctx context.Context, srcLink *proton.Link, dstParentLink *proton.Link, dstName string) error {
	return protonDrive.moveLink(ctx, srcLink, dstParentLink, dstName)
}

func (protonDrive *ProtonDrive) moveLink(ctx context.Context, srcLink *proton.Link, dstParentLink *proton.Link, dstName string) error {
	// we are moving the srcLink to under dstParentLink, with name dstName
	req := proton.MoveLinkReq{
		ParentLinkID:     dstParentLink.LinkID,
		OriginalHash:     srcLink.Hash,
		SignatureAddress: protonDrive.signatureAddress,
	}

	dstParentKR, err := protonDrive.getLinkKR(ctx, dstParentLink)
	if err != nil {
		return err
	}

	err = req.SetName(dstName, protonDrive.AddrKR, dstParentKR)
	if err != nil {
		return err
	}

	dstParentHashKey, err := dstParentLink.GetHashKey(dstParentKR)
	if err != nil {
		return err
	}
	err = req.SetHash(dstName, dstParentHashKey)
	if err != nil {
		return err
	}

	srcParentKR, err := protonDrive.getLinkKRByID(ctx, srcLink.ParentLinkID)
	if err != nil {
		return err
	}
	nodePassphrase, err := reencryptKeyPacket(srcParentKR, dstParentKR, protonDrive.AddrKR, srcLink.NodePassphrase)
	if err != nil {
		return err
	}
	req.NodePassphrase = nodePassphrase
	req.NodePassphraseSignature = srcLink.NodePassphraseSignature

	protonDrive.removeLinkIDFromCache(srcLink.LinkID, false)

	// TODO: disable cache when move is in action?
	// because there might be the case where others read for the same link currently being move -> race condition
	// argument: cache itself is already outdated in a sense, as we don't even have event system (even if we have, it's still outdated...)
	err = protonDrive.c.MoveLink(ctx, protonDrive.MainShare.ShareID, srcLink.LinkID, req)
	if err != nil {
		return err
	}

	time.Sleep(5 * time.Second)

	return nil
}
