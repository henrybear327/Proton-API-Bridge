package proton_api_bridge

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

/*
Observation: file name is unique, since it's checked (by hash) on the server
*/

func (protonDrive *ProtonDrive) SearchByNameRecursivelyFromRoot(ctx context.Context, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	var linkType proton.LinkType
	if isFolder {
		linkType = proton.LinkTypeFolder
	} else {
		linkType = proton.LinkTypeFile
	}
	return protonDrive.searchByNameRecursively(ctx, protonDrive.MainShareKR, protonDrive.RootLink, targetName, linkType, listAllActiveOrDraftFiles)
}

func (protonDrive *ProtonDrive) SearchByNameRecursivelyByID(ctx context.Context, folderLinkID string, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	folderLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, folderLinkID)
	if err != nil {
		return nil, err
	}

	var linkType proton.LinkType
	if isFolder {
		linkType = proton.LinkTypeFolder
	} else {
		linkType = proton.LinkTypeFile
	}

	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}
	folderKeyRing, err := protonDrive.getNodeKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}
	return protonDrive.searchByNameRecursively(ctx, folderKeyRing, &folderLink, targetName, linkType, listAllActiveOrDraftFiles)
}

func (protonDrive *ProtonDrive) SearchByNameRecursively(ctx context.Context, folderLink *proton.Link, targetName string, isFolder bool, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	var linkType proton.LinkType
	if isFolder {
		linkType = proton.LinkTypeFolder
	} else {
		linkType = proton.LinkTypeFile
	}

	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}
	folderKeyRing, err := protonDrive.getNodeKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}
	return protonDrive.searchByNameRecursively(ctx, folderKeyRing, folderLink, targetName, linkType, listAllActiveOrDraftFiles)
}

func (protonDrive *ProtonDrive) searchByNameRecursively(
	ctx context.Context,
	parentNodeKR *crypto.KeyRing,
	link *proton.Link,
	targetName string,
	linkType proton.LinkType,
	listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	if listAllActiveOrDraftFiles {
		if link.State != proton.LinkStateActive && link.State != proton.LinkStateDraft {
			return nil, nil
		}
	} else if link.State != proton.LinkStateActive {
		return nil, nil
	}

	name, err := link.GetName(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}

	if link.Type == linkType && name == targetName {
		return link, nil
	}

	if link.Type == proton.LinkTypeFolder {
		childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, link.LinkID, true)
		if err != nil {
			return nil, err
		}
		// log.Printf("childrenLinks len = %v, %#v", len(childrenLinks), childrenLinks)

		// get current node's keyring
		linkKR, err := link.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}
		defer linkKR.ClearPrivateParams()

		for _, childLink := range childrenLinks {
			ret, err := protonDrive.searchByNameRecursively(ctx, linkKR, &childLink, targetName, linkType, listAllActiveOrDraftFiles)
			if err != nil {
				return nil, err
			}

			if ret != nil {
				return ret, nil
			}
		}
	}

	return nil, nil
}

// if the target isn't found, nil will be returned for both return values
func (protonDrive *ProtonDrive) SearchByNameInFolderByID(ctx context.Context,
	folderLinkID string,
	targetName string,
	searchForFile, searchForFolder, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	folderLink, err := protonDrive.c.GetLink(ctx, protonDrive.MainShare.ShareID, folderLinkID)
	if err != nil {
		return nil, err
	}

	return protonDrive.SearchByNameInFolder(ctx, &folderLink, targetName, searchForFile, searchForFolder, listAllActiveOrDraftFiles)
}

func (protonDrive *ProtonDrive) SearchByNameInFolder(
	ctx context.Context,
	folderLink *proton.Link,
	targetName string,
	searchForFile, searchForFolder, listAllActiveOrDraftFiles bool) (*proton.Link, error) {
	if !searchForFile && !searchForFolder {
		// nothing to search
		return nil, nil
	}

	// we search all folders and files within this designated folder only
	if folderLink.Type != proton.LinkTypeFolder {
		return nil, ErrLinkTypeMustToBeFolderType
	}

	if folderLink.State != proton.LinkStateActive {
		// we only search in the active folders
		return nil, nil
	}

	parentNodeKR, err := protonDrive.getNodeKRByID(ctx, folderLink.ParentLinkID)
	if err != nil {
		return nil, err
	}

	// get current node's keyring
	folderLinkKR, err := folderLink.GetKeyRing(parentNodeKR, protonDrive.AddrKR)
	if err != nil {
		return nil, err
	}
	defer folderLinkKR.ClearPrivateParams()

	childrenLinks, err := protonDrive.c.ListChildren(ctx, protonDrive.MainShare.ShareID, folderLink.LinkID, true)
	if err != nil {
		return nil, err
	}
	for _, childLink := range childrenLinks {
		if listAllActiveOrDraftFiles {
			if childLink.State != proton.LinkStateActive && childLink.State != proton.LinkStateDraft {
				continue
			}
		} else if childLink.State != proton.LinkStateActive {
			continue
		}

		name, err := childLink.GetName(folderLinkKR, protonDrive.AddrKR)
		if err != nil {
			return nil, err
		}

		if searchForFile && childLink.Type == proton.LinkTypeFile && name == targetName {
			return &childLink, nil
		} else if searchForFolder && childLink.Type == proton.LinkTypeFolder && name == targetName {
			return &childLink, nil
		}
	}

	return nil, nil
}
